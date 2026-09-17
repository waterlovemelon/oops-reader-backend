package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/tts"
)

// The routes are registered the way main.go does, minus the JWT middleware: the
// account id is put in the context directly. The TTS provider is a real MiMo
// provider pointed at a stand-in upstream, so the status code under test comes
// from the production code path.
func newTTSStreamTestServer(t *testing.T, upstream http.HandlerFunc) *httptest.Server {
	t.Helper()
	upstreamServer := httptest.NewServer(upstream)
	t.Cleanup(upstreamServer.Close)

	service := tts.NewService(tts.Config{DefaultProvider: "mimo"}, nil)
	service.Register(tts.NewMiMoProvider(tts.MiMoConfig{
		APIKey:  "test-key",
		BaseURL: upstreamServer.URL,
		Model:   "mimo-v2.5-tts",
	}))
	handler := NewTTSHandler(service)
	router := gin.New()
	routes := router.Group("/v1/tts")
	routes.Use(func(c *gin.Context) { c.Set("user_id", testShelfUserID) })
	routes.POST("/stream", handler.StreamSynthesize)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func postStream(t *testing.T, server *httptest.Server) *http.Response {
	t.Helper()
	response, err := server.Client().Post(server.URL+"/v1/tts/stream", "application/json",
		strings.NewReader(`{"text":"测试","voice":"mimo_default","format":"pcm16"}`))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

// An upstream that carries no audio must not reach the client as `200 OK` with
// an empty body: that is indistinguishable from a silent chunk, and it used to
// abort playback of a chapter that was already halfway through.
func TestStreamSynthesizeReportsAnUpstreamWithoutAudioAsBadGateway(t *testing.T) {
	server := newTTSStreamTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	response := postStream(t, server)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("POST /v1/tts/stream status = %d, want 502 (body=%s)", response.StatusCode, body)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("502 body is not JSON: %v (%s)", err, body)
	}
	if message, _ := decoded["error"].(string); !strings.Contains(message, "no audio") {
		t.Fatalf("502 error = %q, want it to name the missing audio", message)
	}
}

func TestStreamSynthesizeForwardsUpstreamAudio(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	server := newTTSStreamTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		payload, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"audio": map[string]string{"data": base64.StdEncoding.EncodeToString(pcm)},
				},
			}},
		})
		_, _ = w.Write(append([]byte("data: "), append(payload, '\n', '\n')...))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	response := postStream(t, server)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /v1/tts/stream status = %d, want 200 (body=%s)", response.StatusCode, body)
	}
	if got := response.Header.Get("X-Audio-Format"); got != "pcm16" {
		t.Fatalf("X-Audio-Format = %q, want pcm16", got)
	}
	if string(body) != string(pcm) {
		t.Fatalf("streamed body = %v, want %v", body, pcm)
	}
}
