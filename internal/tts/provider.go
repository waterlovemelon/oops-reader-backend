package tts

import "context"

// SynthesizeRequest holds parameters for a single TTS synthesis call.
type SynthesizeRequest struct {
	Text        string `json:"text"`
	Voice       string `json:"voice"`
	Format      string `json:"format,omitempty"` // "mp3", "wav", "ogg", "pcm16" — provider-specific
	Rate        int    `json:"rate"`              // percent offset from 1.0, e.g. 0, +20, -10
	Pitch       int    `json:"pitch"`             // percent offset from 1.0
	Volume      int    `json:"volume"`            // percent offset from 1.0
	StylePrompt string `json:"stylePrompt,omitempty"`
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

// ProviderCapabilities describes what a TTS provider can do.
type ProviderCapabilities struct {
	Synthesize       bool   `json:"synthesize"`
	Stream           bool   `json:"stream"`
	StreamLowLatency bool   `json:"streamLowLatency"`
	StreamFormat     string `json:"streamFormat,omitempty"`
	SampleRate       int    `json:"sampleRate,omitempty"`
	Channels         int    `json:"channels,omitempty"`
}

// ProviderInfo is a lightweight descriptor returned by the providers list API.
type ProviderInfo struct {
	Name         string              `json:"name"`
	Label        string              `json:"label"`
	Enabled      bool                `json:"enabled"`
	Capabilities ProviderCapabilities `json:"capabilities,omitempty"`
}

// TTSProvider is the interface every TTS backend must implement.
type TTSProvider interface {
	Name() string
	Label() string
	DefaultVoice() string
	Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error)
	ListVoices(ctx context.Context, locale string) ([]Voice, error)
}

// StreamingTTSProvider is an optional interface for providers that support streaming.
// Providers that implement this interface can deliver audio chunks progressively.
type StreamingTTSProvider interface {
	StreamSynthesize(ctx context.Context, req SynthesizeRequest) (*StreamSynthesizeResponse, error)
}

// StreamSynthesizeResponse holds the streaming audio response.
type StreamSynthesizeResponse struct {
	Format     string
	SampleRate int
	Channels   int
	Chunks     <-chan AudioChunk
	Err        <-chan error
}

// AudioChunk represents a single chunk of audio data in a stream.
type AudioChunk struct {
	Data []byte
}
