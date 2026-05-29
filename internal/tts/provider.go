package tts

import "context"

// SynthesizeRequest holds parameters for a single TTS synthesis call.
type SynthesizeRequest struct {
	Text   string `json:"text"`
	Voice  string `json:"voice"`
	Format string `json:"format,omitempty"` // "mp3", "wav", "ogg" — provider-specific
	Rate   int    `json:"rate"`             // percent offset from 1.0, e.g. 0, +20, -10
	Pitch  int    `json:"pitch"`            // percent offset from 1.0
	Volume int    `json:"volume"`           // percent offset from 1.0
}

// SynthesizeResponse is the audio result from a TTS provider.
type SynthesizeResponse struct {
	AudioData []byte
	Format    string // "mp3", "wav", "ogg"
}

// Voice describes a single TTS voice offered by a provider.
type Voice struct {
	Value          string            `json:"value"`
	Label          string            `json:"label"`
	Locale         string            `json:"locale"`
	Gender         string            `json:"gender"`
	Names          map[string]string `json:"names,omitempty"`
	Characteristics VoiceCharacteristics `json:"characteristics,omitempty"`
}

// VoiceCharacteristics groups personality and category tags.
type VoiceCharacteristics struct {
	Personalities map[string][]string `json:"personalities,omitempty"`
	Categories    map[string][]string `json:"categories,omitempty"`
}

// ProviderInfo is a lightweight descriptor returned by the providers list API.
type ProviderInfo struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// TTSProvider is the interface every TTS backend must implement.
type TTSProvider interface {
	Name() string
	Label() string
	Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error)
	ListVoices(ctx context.Context, locale string) ([]Voice, error)
}
