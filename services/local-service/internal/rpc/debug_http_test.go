package rpc

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestHandleDebugEventsReturnsQueuedNotifications verifies that queued
// notifications can be fetched through the debug events endpoint.
func TestHandleDebugEventsReturnsQueuedNotifications(t *testing.T) {
	server := newTestServer()
	result, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "直接总结这段文字",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/events?task_id="+taskID, nil)
	server.handleDebugEvents(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	items := payload["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected queued notifications to be returned")
	}
}

func TestHandleHTTPRPCAllowsLoopbackStyleOrigins(t *testing.T) {
	server := newTestServer()
	origins := []string{
		"http://localhost:5173",
		"https://127.0.0.1:5173",
		"tauri://localhost",
		"https://tauri.localhost",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("OPTIONS", "/rpc", nil)
			request.Header.Set("Origin", origin)

			server.handleHTTPRPC(recorder, request)

			if recorder.Code != 204 {
				t.Fatalf("expected 204 status, got %d", recorder.Code)
			}
			if recorder.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatalf("expected CORS allow origin %q, got %q", origin, recorder.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestHandleHTTPRPCRejectsNonLoopbackOrigins(t *testing.T) {
	server := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("OPTIONS", "/rpc", nil)
	request.Header.Set("Origin", "https://example.com")

	server.handleHTTPRPC(recorder, request)

	if recorder.Code != 204 {
		t.Fatalf("expected 204 status, got %d", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no CORS allow origin for non-loopback request, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}
