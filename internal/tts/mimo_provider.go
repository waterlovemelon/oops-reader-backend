package tts

import (
	"bufio"
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

func (p *MiMoProvider) Name() string         { return "mimo" }
func (p *MiMoProvider) Label() string        { return "Xiaomi MiMo TTS" }
func (p *MiMoProvider) DefaultVoice() string { return "mimo_default" }

// Capabilities returns the low-latency features supported by MiMo TTS.
func (p *MiMoProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Synthesize:       true,
		Stream:           true,
		StreamLowLatency: p.model == "mimo-v2.5-tts",
		StreamFormat:     "pcm16",
		SampleRate:       24000,
		Channels:         1,
	}
}

// mimoRequest is the OpenAI-compatible chat completions request body.
type mimoRequest struct {
	Model    string        `json:"model"`
	Messages []mimoMessage `json:"messages"`
	Audio    mimoAudioSpec `json:"audio"`
	Stream   bool          `json:"stream,omitempty"`
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

type mimoStreamResponse struct {
	Choices []struct {
		Delta struct {
			Audio struct {
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// StreamSynthesize forwards MiMo's SSE audio deltas as decoded PCM bytes.
func (p *MiMoProvider) StreamSynthesize(ctx context.Context, req SynthesizeRequest, write func([]byte) error) (StreamMetadata, error) {
	if p.apiKey == "" {
		return StreamMetadata{}, fmt.Errorf("mimo tts: api_key not configured")
	}
	if write == nil {
		return StreamMetadata{}, fmt.Errorf("mimo tts: stream writer is nil")
	}

	voice := req.Voice
	if voice == "" {
		voice = p.DefaultVoice()
	}
	stylePrompt := req.StylePrompt
	if stylePrompt == "" {
		stylePrompt = "请用自然流畅的语调朗读以下内容。"
	}
	body, err := json.Marshal(mimoRequest{
		Model: p.model,
		Messages: []mimoMessage{
			{Role: "user", Content: stylePrompt},
			{Role: "assistant", Content: req.Text},
		},
		Audio:  mimoAudioSpec{Format: "pcm16", Voice: voice},
		Stream: true,
	})
	if err != nil {
		return StreamMetadata{}, fmt.Errorf("mimo tts: marshal stream request: %w", err)
	}

	url := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return StreamMetadata{}, fmt.Errorf("mimo tts: build stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return StreamMetadata{}, fmt.Errorf("mimo tts: stream request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return StreamMetadata{}, fmt.Errorf("mimo tts: stream status %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return StreamMetadata{Format: "pcm16", SampleRate: 24000, Channels: 1}, nil
		}

		var chunk mimoStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return StreamMetadata{}, fmt.Errorf("mimo tts: decode stream event: %w", err)
		}
		if chunk.Error != nil {
			return StreamMetadata{}, fmt.Errorf("mimo tts: stream api error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Audio.Data == "" {
			continue
		}
		pcm, err := base64.StdEncoding.DecodeString(chunk.Choices[0].Delta.Audio.Data)
		if err != nil {
			return StreamMetadata{}, fmt.Errorf("mimo tts: decode stream audio: %w", err)
		}
		if len(pcm) > 0 {
			if err := write(pcm); err != nil {
				return StreamMetadata{}, fmt.Errorf("mimo tts: write stream audio: %w", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return StreamMetadata{}, fmt.Errorf("mimo tts: read stream: %w", err)
	}
	return StreamMetadata{Format: "pcm16", SampleRate: 24000, Channels: 1}, nil
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
		{Role: "user", Content: req.StylePrompt},
		{Role: "assistant", Content: req.Text},
	}
	if messages[0].Content == "" {
		messages[0].Content = "请用自然流畅的语调朗读以下内容。"
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
