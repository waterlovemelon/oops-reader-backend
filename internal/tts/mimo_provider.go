package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MiMoProvider implements TTSProvider for Xiaomi MiMo-V2.5-TTS series.
type MiMoProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// MiMoConfig holds connection settings for the MiMo TTS API.
type MiMoConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

func NewMiMoProvider(cfg MiMoConfig) *MiMoProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://token-plan-cn.xiaomimimo.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "mimo-v2.5-tts"
	}
	return &MiMoProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *MiMoProvider) Name() string  { return "mimo" }
func (p *MiMoProvider) Label() string { return "Xiaomi MiMo TTS" }

// mimoRequest is the OpenAI-compatible chat completions request body.
type mimoRequest struct {
	Model    string         `json:"model"`
	Messages []mimoMessage  `json:"messages"`
	Audio    mimoAudioSpec  `json:"audio"`
}

type mimoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mimoAudioSpec struct {
	Format string `json:"format"`
	Voice  string `json:"voice,omitempty"`
}

// mimoResponse is the OpenAI-compatible chat completions response.
type mimoResponse struct {
	Choices []struct {
		Message struct {
			Audio struct {
				Data string `json:"data"` // base64-encoded audio
			} `json:"audio"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (p *MiMoProvider) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("mimo tts: api_key not configured")
	}

	voice := req.Voice
	if voice == "" {
		voice = "mimo_default"
	}

	format := "wav"
	if req.Format != "" {
		format = req.Format
	}

	messages := []mimoMessage{
		{Role: "user", Content: "请用自然流畅的语调朗读以下内容。"},
		{Role: "assistant", Content: req.Text},
	}

	body := mimoRequest{
		Model:    p.model,
		Messages: messages,
		Audio: mimoAudioSpec{
			Format: format,
			Voice:  voice,
		},
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mimo tts: marshal request: %w", err)
	}

	url := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("mimo tts: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mimo tts: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mimo tts: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mimo tts: status %d: %s", resp.StatusCode, string(respBody))
	}

	var mimoResp mimoResponse
	if err := json.Unmarshal(respBody, &mimoResp); err != nil {
		return nil, fmt.Errorf("mimo tts: decode response: %w", err)
	}

	if mimoResp.Error != nil {
		return nil, fmt.Errorf("mimo tts: api error: %s", mimoResp.Error.Message)
	}

	if len(mimoResp.Choices) == 0 {
		return nil, fmt.Errorf("mimo tts: no audio data in response")
	}

	audioB64 := mimoResp.Choices[0].Message.Audio.Data
	if audioB64 == "" {
		return nil, fmt.Errorf("mimo tts: empty audio data in response")
	}

	audioData, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return nil, fmt.Errorf("mimo tts: decode audio base64: %w", err)
	}

	return &SynthesizeResponse{
		AudioData: audioData,
		Format:    format,
	}, nil
}

// ListVoices returns the preset voices for MiMo TTS.
func (p *MiMoProvider) ListVoices(ctx context.Context, locale string) ([]Voice, error) {
	voices := []Voice{
		{Value: "mimo_default", Label: "MiMo-默认", Locale: "zh-CN", Gender: "female"},
		{Value: "冰糖", Label: "冰糖", Locale: "zh-CN", Gender: "female"},
		{Value: "茉莉", Label: "茉莉", Locale: "zh-CN", Gender: "female"},
		{Value: "苏打", Label: "苏打", Locale: "zh-CN", Gender: "male"},
		{Value: "白桦", Label: "白桦", Locale: "zh-CN", Gender: "male"},
		{Value: "Mia", Label: "Mia", Locale: "en", Gender: "female"},
		{Value: "Chloe", Label: "Chloe", Locale: "en", Gender: "female"},
		{Value: "Milo", Label: "Milo", Locale: "en", Gender: "male"},
		{Value: "Dean", Label: "Dean", Locale: "en", Gender: "male"},
	}

	if locale != "" {
		filtered := make([]Voice, 0)
		for _, v := range voices {
			if strings.HasPrefix(v.Locale, locale) || strings.HasPrefix(locale, v.Locale) {
				filtered = append(filtered, v)
			}
		}
		return filtered, nil
	}

	return voices, nil
}
