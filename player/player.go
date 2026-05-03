package player

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/ebitengine/oto/v3"
)

var otoCtx *oto.Context

func Init() error {
	op := &oto.NewContextOptions{
		SampleRate:   24000,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return fmt.Errorf("oto.NewContext: %w", err)
	}
	<-ready
	otoCtx = ctx
	return nil
}

func PlayWAV(wavData []byte) error {
	if otoCtx == nil {
		return fmt.Errorf("player not initialized: call Init() first")
	}

	pcm, err := extractPCM(wavData)
	if err != nil {
		return fmt.Errorf("wav parse: %w", err)
	}

	player := otoCtx.NewPlayer(bytes.NewReader(pcm))
	player.Play()
	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}
	return player.Close()
}

func extractPCM(wavData []byte) ([]byte, error) {
	r := bytes.NewReader(wavData)

	var riff [4]byte
	if err := binary.Read(r, binary.LittleEndian, &riff); err != nil {
		return nil, err
	}
	if string(riff[:]) != "RIFF" {
		return nil, fmt.Errorf("not a RIFF file")
	}

	// RIFFサイズ(4) + "WAVE"(4) をスキップして最初のチャンクへ
	if _, err := r.Seek(12, io.SeekStart); err != nil {
		return nil, err
	}

	for {
		var chunkID [4]byte
		var chunkSize uint32
		if err := binary.Read(r, binary.LittleEndian, &chunkID); err != nil {
			return nil, fmt.Errorf("dataチャンクが見つかりません")
		}
		if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
			return nil, err
		}

		if string(chunkID[:]) == "data" {
			pcm := make([]byte, chunkSize)
			if _, err := r.Read(pcm); err != nil {
				return nil, err
			}
			return pcm, nil
		}

		// このチャンクをスキップ
		if _, err := r.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
			return nil, err
		}
	}
}
