package recorder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"
	"golang.org/x/term"
)

const (
	sampleRate    = 16000
	channels      = 1
	chunkSize     = 1024
	maxDuration   = 30 * time.Second
	silenceThresh = 500
	silenceDur    = 1500 * time.Millisecond
)

// captureAudio はマイクからチャンクを読み続け、各チャンクを onChunk に渡す。
// stopFn が true を返すか maxDur を超えると停止する。
func captureAudio(stopFn func() bool, maxDur time.Duration, onChunk func([]int16)) error {
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("portaudio.Initialize: %w", err)
	}
	defer portaudio.Terminate()

	buf := make([]int16, chunkSize)
	stream, err := portaudio.OpenDefaultStream(channels, 0, float64(sampleRate), len(buf), buf)
	if err != nil {
		return fmt.Errorf("OpenDefaultStream: %w", err)
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return fmt.Errorf("stream.Start: %w", err)
	}
	defer stream.Stop()

	deadline := time.Now().Add(maxDur)
	for time.Now().Before(deadline) {
		if err := stream.Read(); err != nil {
			return fmt.Errorf("stream.Read: %w", err)
		}

		chunk := make([]int16, len(buf))
		copy(chunk, buf)
		onChunk(chunk)

		if stopFn != nil && stopFn() {
			break
		}
	}

	return nil
}

func encodeWAV(pcm []int16) ([]byte, error) {
	var buf bytes.Buffer

	numSamples := len(pcm)
	dataSize := numSamples * 2
	byteRate := sampleRate * channels * 2
	blockAlign := channels * 2

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(channels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	for _, s := range pcm {
		binary.Write(&buf, binary.LittleEndian, s)
	}

	return buf.Bytes(), nil
}

func rms(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func RecordPushToTalk() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("term.MakeRaw: %w", err)
	}
	defer term.Restore(fd, oldState)

	fmt.Print("Enterを押している間だけ録音します。離すと送信します...\r\n")

	waitForKey(13)
	fmt.Print("録音中...\r\n")

	released := make(chan struct{})
	go func() {
		waitForKey(13)
		close(released)
	}()

	stopFn := func() bool {
		select {
		case <-released:
			return true
		default:
			return false
		}
	}

	var pcm []int16
	if err := captureAudio(stopFn, maxDuration, func(chunk []int16) {
		pcm = append(pcm, chunk...)
	}); err != nil {
		return nil, err
	}

	return encodeWAV(pcm)
}

func waitForKey(key byte) {
	buf := make([]byte, 1)
	for {
		os.Stdin.Read(buf)
		if buf[0] == key {
			return
		}
	}
}

func RecordVAD() ([]byte, error) {
	fmt.Println("話し始めてください（無音が続くと自動で送信）...")

	var allPCM []int16
	silenceStart := time.Time{}
	speaking := false

	stopFn := func() bool {
		if len(allPCM) == 0 {
			return false
		}

		windowSize := chunkSize
		if len(allPCM) < windowSize {
			windowSize = len(allPCM)
		}
		window := allPCM[len(allPCM)-windowSize:]
		level := rms(window)

		if level > silenceThresh {
			speaking = true
			silenceStart = time.Time{}
		} else if speaking {
			if silenceStart.IsZero() {
				silenceStart = time.Now()
			} else if time.Since(silenceStart) > silenceDur {
				return true
			}
		}
		return false
	}

	if err := captureAudio(stopFn, maxDuration, func(chunk []int16) {
		allPCM = append(allPCM, chunk...)
	}); err != nil {
		return nil, err
	}

	return encodeWAV(allPCM)
}
