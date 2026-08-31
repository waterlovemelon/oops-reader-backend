package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiMoProviderStreamSynthesizeForwardsAudioBeforeUpstreamCompletes(t *testing.T) {
	firstPCM := []byte{0x01, 0x02, 0x03, 0x04}
	secondPCM := []byte{0x05, 0x06}
	upstreamDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		var request mimoRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !request.Stream || request.Audio.Format != "pcm16" || request.Model != "mimo-v2.5-tts" {
			t.Fatalf("unexpected stream request: %+v", request)
		}
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("api-key = %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		writeEvent := func(pcm []byte) {
			payload, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{
						"audio": map[string]string{"data": base64.StdEncoding.EncodeToString(pcm)},
					},
				}},
			})
			_, _ = w.Write(append([]byte("data: "), append(payload, '\n', '\n')...))
			flusher.Flush()
		}
		writeEvent(firstPCM)
		time.Sleep(50 * time.Millisecond)
		writeEvent(secondPCM)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
		close(upstreamDone)
	}))
	defer server.Close()

	provider := NewMiMoProvider(MiMoConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "mimo-v2.5-tts",
	})

	var received []byte
	firstChunk := make(chan struct{})
	result := make(chan struct {
		metadata StreamMetadata
		err      error
	}, 1)
	go func() {
		metadata, err := provider.StreamSynthesize(context.Background(), SynthesizeRequest{
			Text: "测试流式播放",
		}, func(chunk []byte) error {
			if len(received) == 0 {
				close(firstChunk)
			}
			received = append(received, chunk...)
			return nil
		})
		result <- struct {
			metadata StreamMetadata
			err      error
		}{metadata: metadata, err: err}
	}()
	select {
	case <-firstChunk:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first audio chunk")
	}
	output := <-result
	if output.err != nil {
		t.Fatalf("StreamSynthesize() error = %v", output.err)
	}
	if output.metadata.Format != "pcm16" || output.metadata.SampleRate != 24000 || output.metadata.Channels != 1 {
		t.Fatalf("metadata = %+v", output.metadata)
	}
	if string(received) != string(append(firstPCM, secondPCM...)) {
		t.Fatalf("received PCM = %v, want %v", received, append(firstPCM, secondPCM...))
	}
}
