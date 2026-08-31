package tts

import "context"

// SynthesizeRequest holds parameters for a single TTS synthesis call.
type SynthesizeRequest struct {
	Text        string `json:"text"`
	Voice       string `json:"voice"`
	StylePrompt string `json:"stylePrompt,omitempty"`
	Format      string `json:"format,omitempty"` // "mp3", "wav", "ogg" — provider-specific
	Rate        int    `json:"rate"`             // percent offset from 1.0, e.g. 0, +20, -10
	Pitch       int    `json:"pitch"`            // percent offset from 1.0
	Volume      int    `json:"volume"`           // percent offset from 1.0
}

// StreamingTTSProvider is an optional capability for providers that can emit
// audio progressively. The callback is invoked in response order and should
// return an error to stop the upstream request.
type StreamingTTSProvider interface {
	StreamSynthesize(ctx context.Context, req SynthesizeRequest, write func([]byte) error) (StreamMetadata, error)
}

// StreamMetadata describes the raw audio bytes emitted by a streaming provider.
type StreamMetadata struct {
	Format     string
	SampleRate int
	Channels   int
}

// SynthesizeResponse is the audio result from a TTS provider.
type SynthesizeResponse struct {
	AudioData []byte
	Format    string // "mp3", "wav", "ogg"
}

// Voice describes a single TTS voice offered by a provider.
type Voice struct {
	Value           string               `json:"value"`
	Label           string               `json:"label"`
	Locale          string               `json:"locale"`
	Gender          string               `json:"gender"`
	Names           map[string]string    `json:"names,omitempty"`
	Characteristics VoiceCharacteristics `json:"characteristics,omitempty"`
}

// VoiceCharacteristics groups personality and category tags.
type VoiceCharacteristics struct {
	Personalities map[string][]string `json:"personalities,omitempty"`
	Categories    map[string][]string `json:"categories,omitempty"`
}

// ProviderInfo is a lightweight descriptor returned by the providers list API.
type ProviderInfo struct {
	Name         string               `json:"name"`
	Label        string               `json:"label"`
	Enabled      bool                 `json:"enabled"`
	Capabilities ProviderCapabilities `json:"capabilities,omitempty"`
}

// ProviderCapabilities describes optional provider features exposed to clients.
type ProviderCapabilities struct {
	Synthesize       bool   `json:"synthesize"`
	Stream           bool   `json:"stream"`
	StreamLowLatency bool   `json:"streamLowLatency,omitempty"`
	StreamFormat     string `json:"streamFormat,omitempty"`
	SampleRate       int    `json:"sampleRate,omitempty"`
	Channels         int    `json:"channels,omitempty"`
}

// TTSProvider is the interface every TTS backend must implement.
type TTSProvider interface {
	Name() string
	Label() string
	Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error)
	ListVoices(ctx context.Context, locale string) ([]Voice, error)
}
