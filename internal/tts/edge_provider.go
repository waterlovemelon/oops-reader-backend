package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EdgeProvider proxies requests to an Edge TTS compatible server.
type EdgeProvider struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// EdgeConfig holds connection settings for the Edge TTS server.
type EdgeConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Token   string `mapstructure:"token"`
}

func NewEdgeProvider(cfg EdgeConfig) *EdgeProvider {
	return &EdgeProvider{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *EdgeProvider) Name() string  { return "edge" }
func (p *EdgeProvider) Label() string { return "Edge TTS" }

func (p *EdgeProvider) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("edge tts: base_url not configured")
	}

	fullURL := strings.TrimRight(p.baseURL, "/") + "/api/text-to-speech?" + url.Values{
		"voice":  {req.Voice},
		"text":   {req.Text},
		"rate":   {formatPercent(req.Rate)},
		"pitch":  {formatPercent(req.Pitch)},
		"volume": {formatPercent(req.Volume)},
	}.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("edge tts: build request: %w", err)
	}
	if p.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("edge tts: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("edge tts: status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("edge tts: read response: %w", err)
	}

	format := "mp3"
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		format = mimeToFormat(ct)
	}

	return &SynthesizeResponse{
		AudioData: data,
		Format:    format,
	}, nil
}

func (p *EdgeProvider) ListVoices(ctx context.Context, locale string) ([]Voice, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("edge tts: base_url not configured")
	}

	fullURL := strings.TrimRight(p.baseURL, "/") + "/api/voices"
	if locale != "" {
		fullURL += "?" + url.Values{"locale": {locale}}.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("edge tts: build request: %w", err)
	}
	if p.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("edge tts: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("edge tts: status %d: %s", resp.StatusCode, string(body))
	}

	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("edge tts: decode voices: %w", err)
	}

	voices := make([]Voice, 0, len(raw))
	for _, r := range raw {
		var v edgeVoice
		if err := json.Unmarshal(r, &v); err != nil {
			continue
		}
		voices = append(voices, v.toVoice())
	}
	return voices, nil
}

// edgeVoice matches the upstream Edge TTS voice JSON shape.
type edgeVoice struct {
	Value          string `json:"value"`
	Label          string `json:"label"`
	Locale         string `json:"locale"`
	Gender         string `json:"gender"`
	Names          map[string]string `json:"names"`
	Characteristics struct {
		Personalities map[string][]string `json:"personalities"`
		Categories    map[string][]string `json:"categories"`
	} `json:"characteristics"`
}

func (v *edgeVoice) toVoice() Voice {
	return Voice{
		Value:  v.Value,
		Label:  v.Label,
		Locale: v.Locale,
		Gender: v.Gender,
		Names:  v.Names,
		Characteristics: VoiceCharacteristics{
			Personalities: v.Characteristics.Personalities,
			Categories:    v.Characteristics.Categories,
		},
	}
}

func formatPercent(offset int) string {
	return fmt.Sprintf("%d", offset)
}

func mimeToFormat(mime string) string {
	switch {
	case mime == "audio/mpeg" || mime == "audio/mp3":
		return "mp3"
	case mime == "audio/wav" || mime == "audio/x-wav":
		return "wav"
	case mime == "audio/ogg":
		return "ogg"
	case mime == "audio/aac":
		return "aac"
	default:
		return "mp3"
	}
}
