package EdgeClient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSONSuccess(t *testing.T) {
	var gotMethod string
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAPIKey = r.Header.Get("X-Edge-API-Key")
		_ = json.NewEncoder(w).Encode(Response[map[string]any]{
			Code:    1000,
			Message: "success",
			Data: map[string]any{
				"taskId": "edge-task-1",
				"status": "queued",
			},
		})
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client()}
	var result TaskStartResponse
	err := client.DoJSON(context.Background(), server.URL, http.MethodPost, "/api/v1/edge/browser-envs/env-1/run", "secret", map[string]any{"forceRecreate": true}, &result)
	if err != nil {
		t.Fatalf("DoJSON returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method=%s", gotMethod)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("api key header=%s", gotAPIKey)
	}
	if result.TaskID != "edge-task-1" || result.Status != "queued" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDoJSONBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Response[json.RawMessage]{
			Code:    1003,
			Message: "环境包处于 error，请先 revalidate",
		})
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client()}
	err := client.DoJSON(context.Background(), server.URL, http.MethodPost, "/api/v1/edge/browser-envs/env-1/run", "", nil, nil)
	var edgeErr *EdgeError
	if !errors.As(err, &edgeErr) {
		t.Fatalf("expected EdgeError, got %T %v", err, err)
	}
	if edgeErr.EdgeCode != 1003 || edgeErr.EdgeMessage == "" {
		t.Fatalf("unexpected edge error: %+v", edgeErr)
	}
}

func TestDoJSONDoesNotRetryHTTPFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client()}
	err := client.DoJSON(context.Background(), server.URL, http.MethodPost, "/api/v1/edge/browser-envs/env-1/backup", "", nil, nil)
	var edgeErr *EdgeError
	if !errors.As(err, &edgeErr) {
		t.Fatalf("expected EdgeError, got %T %v", err, err)
	}
	if calls != 1 {
		t.Fatalf("EdgeClient must not retry asset actions, calls=%d", calls)
	}
	if edgeErr.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected http status: %+v", edgeErr)
	}
}
