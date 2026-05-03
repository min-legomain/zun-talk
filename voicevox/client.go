package voicevox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const defaultSpeed = 1.1

type voiceParams struct {
	speed      float64
	pitch      float64
	intonation float64
}

var emotionParams = map[string]voiceParams{
	"喜び":  {speed: 1.2, pitch: 0.05, intonation: 1.3},
	"悲しみ": {speed: 0.9, pitch: -0.05, intonation: 0.8},
	"驚き":  {speed: 1.2, pitch: 0.08, intonation: 1.5},
	"怒り":  {speed: 1.15, pitch: -0.03, intonation: 1.4},
}

var defaultParams = voiceParams{speed: defaultSpeed, pitch: 0, intonation: 1.0}

type Client struct {
	baseURL   string
	speakerID int
}

func NewClient(baseURL string, speakerID int) *Client {
	return &Client{baseURL: baseURL, speakerID: speakerID}
}

func (c *Client) SetSpeakerID(id int) {
	c.speakerID = id
}

func (c *Client) Synthesize(text, emotion string) ([]byte, error) {
	query, err := c.audioQuery(text)
	if err != nil {
		return nil, fmt.Errorf("audio_query: %w", err)
	}

	params, ok := emotionParams[emotion]
	if !ok {
		params = defaultParams
	}

	query, err = applyVoiceParams(query, params)
	if err != nil {
		return nil, fmt.Errorf("applyVoiceParams: %w", err)
	}

	return c.synthesis(query)
}

func applyVoiceParams(query json.RawMessage, p voiceParams) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(query, &m); err != nil {
		return nil, err
	}
	set := func(key string, val float64) error {
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		m[key] = b
		return nil
	}
	if err := set("speedScale", p.speed); err != nil {
		return nil, err
	}
	if err := set("pitchScale", p.pitch); err != nil {
		return nil, err
	}
	if err := set("intonationScale", p.intonation); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (c *Client) buildURL(path string, params map[string]string) (string, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("url.Parse: %w", err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) audioQuery(text string) (json.RawMessage, error) {
	endpoint, err := c.buildURL("/audio_query", map[string]string{
		"text":    text,
		"speaker": fmt.Sprintf("%d", c.speakerID),
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(endpoint, "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) synthesis(query json.RawMessage) ([]byte, error) {
	endpoint, err := c.buildURL("/synthesis", map[string]string{
		"speaker": fmt.Sprintf("%d", c.speakerID),
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
