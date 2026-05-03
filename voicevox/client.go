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
