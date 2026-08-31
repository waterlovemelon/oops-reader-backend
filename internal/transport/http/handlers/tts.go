package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
	"github.com/oops-reader/oops-reader-backend/internal/tts"
)

type TTSHandler struct {
	service     *tts.Service
	defaultProv string
}

func NewTTSHandler(service *tts.Service) *TTSHandler {
	return &TTSHandler{
		service:     service,
		defaultProv: service.Default(),
	}
}

// Synthesize proxies a TTS synthesis request to the active provider.
//
//	GET /v1/tts/synthesize?voice=&text=&rate=&pitch=&volume=
//	GET /v1/tts/synthesize/:provider?voice=&text=&rate=&pitch=&volume=
func (h *TTSHandler) Synthesize(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)

	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text parameter is required"})
		return
	}

	providerName := c.Param("provider")
	if providerName == "" {
		p, _, err := h.service.ProviderForUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		providerName = p.Name()
	}

	provider, err := h.service.Provider(providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	voice := c.Query("voice")
	if voice == "" {
		_, prefVoice, _ := h.service.ProviderForUser(c.Request.Context(), userID)
		voice = prefVoice
	}
	// Don't fall back to DefaultVoice here — each provider handles its own default.

	req := tts.SynthesizeRequest{
		Text:   text,
		Voice:  voice,
		Rate:   queryInt(c, "rate", 0),
		Pitch:  queryInt(c, "pitch", 0),
		Volume: queryInt(c, "volume", 0),
	}

	resp, err := provider.Synthesize(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	contentType := formatToMIME(resp.Format)
	c.Data(http.StatusOK, contentType, resp.AudioData)
}

// StreamSynthesize proxies a provider's progressive PCM output to the client.
//
//	POST /v1/tts/stream
//	POST /v1/tts/stream/:provider
func (h *TTSHandler) StreamSynthesize(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)
	var input struct {
		Text        string `json:"text"`
		Voice       string `json:"voice"`
		StylePrompt string `json:"stylePrompt"`
		Rate        int    `json:"rate"`
		Pitch       int    `json:"pitch"`
		Volume      int    `json:"volume"`
		Format      string `json:"format"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if input.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	providerName := c.Param("provider")
	if providerName == "" {
		p, _, err := h.service.ProviderForUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		providerName = p.Name()
	}
	provider, err := h.service.Provider(providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	streamingProvider, ok := provider.(tts.StreamingTTSProvider)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "provider does not support streaming"})
		return
	}

	voice := input.Voice
	if voice == "" {
		_, prefVoice, _ := h.service.ProviderForUser(c.Request.Context(), userID)
		voice = prefVoice
	}

	req := tts.SynthesizeRequest{
		Text: input.Text, Voice: voice, StylePrompt: input.StylePrompt,
		Rate: input.Rate, Pitch: input.Pitch, Volume: input.Volume, Format: input.Format,
	}
	metadata := tts.StreamMetadata{Format: "pcm16", SampleRate: 24000, Channels: 1}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Audio-Format", metadata.Format)
	c.Header("X-Audio-Sample-Rate", strconv.Itoa(metadata.SampleRate))
	c.Header("X-Audio-Channels", strconv.Itoa(metadata.Channels))
	c.Header("X-Audio-Sample-Format", "s16le")
	c.Header("Content-Type", "audio/L16; rate=24000; channels=1")

	flusher, _ := c.Writer.(http.Flusher)
	_, err = streamingProvider.StreamSynthesize(c.Request.Context(), req, func(data []byte) error {
		if len(data) == 0 {
			return nil
		}
		if _, writeErr := c.Writer.Write(data); writeErr != nil {
			return writeErr
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	})
	if err != nil {
		// Headers/body may already have been sent, so only report the error in
		// the body when it is still possible to change the response status.
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
		return
	}
}

// ListVoices returns voices from the active or specified provider.
//
//	GET /v1/tts/voices?locale=
//	GET /v1/tts/voices/:provider?locale=
func (h *TTSHandler) ListVoices(c *gin.Context) {
	userID, _ := middleware.CurrentUserID(c)

	providerName := c.Param("provider")
	if providerName == "" {
		p, _, err := h.service.ProviderForUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		providerName = p.Name()
	}

	provider, err := h.service.Provider(providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	locale := c.Query("locale")
	voices, err := provider.ListVoices(c.Request.Context(), locale)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, voices)
}

// ListProviders returns all registered TTS providers.
//
//	GET /v1/tts/providers
func (h *TTSHandler) ListProviders(c *gin.Context) {
	providers := h.service.ListProviders()
	c.JSON(http.StatusOK, providers)
}

// SelectProvider saves a user's TTS provider and voice preference.
//
//	POST /v1/tts/provider/select
//	Body: { "provider": "edge", "voice": "..." }
func (h *TTSHandler) SelectProvider(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
		return
	}

	var req struct {
		Provider string `json:"provider" binding:"required"`
		Voice    string `json:"voice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if _, err := h.service.Provider(req.Provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SaveUserPref(c.Request.Context(), userID, req.Provider, req.Voice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func queryInt(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	val := 0
	neg := false
	for i, ch := range s {
		if i == 0 && ch == '-' {
			neg = true
			continue
		}
		if ch >= '0' && ch <= '9' {
			val = val*10 + int(ch-'0')
		} else {
			break
		}
	}
	if neg {
		val = -val
	}
	return val
}

func formatToMIME(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "ogg":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	default:
		return "audio/mpeg"
	}
}
