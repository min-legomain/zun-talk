package voicevox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const SpeedScale = 1.1 // 話速（1.0=標準、上げると速い）

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

func (c *Client) Synthesize(text string) ([]byte, error) {
	query, err := c.audioQuery(text)
	if err != nil {
		return nil, fmt.Errorf("audio_query: %w", err)
	}

	query, err = applySpeedScale(query, SpeedScale)
	if err != nil {
		return nil, fmt.Errorf("speedScale: %w", err)
	}

	wav, err := c.synthesis(query)
	if err != nil {
		return nil, fmt.Errorf("synthesis: %w", err)
	}

	return wav, nil
}

func applySpeedScale(query json.RawMessage, speed float64) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(query, &m); err != nil {
		return nil, err
	}
	b, err := json.Marshal(speed)
	if err != nil {
		return nil, err
	}
	m["speedScale"] = b
	return json.Marshal(m)
}

func (c *Client) audioQuery(text string) (json.RawMessage, error) {
	u, _ := url.Parse(c.baseURL + "/audio_query")
	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", fmt.Sprintf("%d", c.speakerID))
	u.RawQuery = q.Encode()

	resp, err := http.Post(u.String(), "application/json", nil)
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
	u, _ := url.Parse(c.baseURL + "/synthesis")
	q := u.Query()
	q.Set("speaker", fmt.Sprintf("%d", c.speakerID))
	u.RawQuery = q.Encode()

	resp, err := http.Post(u.String(), "application/json", bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
