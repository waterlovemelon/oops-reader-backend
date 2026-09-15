package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/tts"
)

type stubVoiceProvider struct{ name string }

func (p stubVoiceProvider) Name() string  { return p.name }
func (p stubVoiceProvider) Label() string { return p.name }
func (p stubVoiceProvider) Synthesize(context.Context, tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	return &tts.SynthesizeResponse{}, nil
}
func (p stubVoiceProvider) ListVoices(context.Context, string) ([]tts.Voice, error) { return nil, nil }

// The routes are registered the way main.go does, minus the JWT middleware: the
// account id is put in the context directly.
func newTTSTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	service := tts.NewService(tts.Config{DefaultProvider: "edge"}, nil)
	service.Register(stubVoiceProvider{name: "edge"})
	handler := NewTTSHandler(service)
	router := gin.New()
	routes := router.Group("/v1/tts")
	routes.Use(func(c *gin.Context) { c.Set("user_id", testShelfUserID) })
	routes.GET("/prefs", handler.GetPrefs)
	routes.POST("/provider/select", handler.SelectProvider)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func TestGetPrefsReportsTheUnconfiguredDefault(t *testing.T) {
	server := newTTSTestServer(t)

	response, err := server.Client().Get(server.URL + "/v1/tts/prefs")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/tts/prefs status = %d, want 200", response.StatusCode)
	}
	decoded := map[string]any{}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("response has no data object: %v", decoded)
	}
	if data["provider"] != "edge" || data["voice"] != "" || data["configured"] != false {
		t.Fatalf("data = %v, want the unconfigured edge default", data)
	}
}

// The provider field became optional so a device can change only the voice;
// an explicitly named, unregistered provider is still rejected.
func TestSelectProviderRejectsAnUnregisteredProvider(t *testing.T) {
	server := newTTSTestServer(t)

	response, err := server.Client().Post(server.URL+"/v1/tts/provider/select", "application/json",
		strings.NewReader(`{"provider":"ghost","voice":"voice-x"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST with an unregistered provider status = %d, want 400", response.StatusCode)
	}
}
