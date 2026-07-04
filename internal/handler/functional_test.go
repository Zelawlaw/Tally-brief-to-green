//go:build functional

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

const baseURL = "http://localhost:8081"

func apiURL(path string) string {
	return baseURL + path
}

func TestFunctionalCreateMemberAndContribute(t *testing.T) {
	member := postJSON(t, apiURL("/members"), map[string]any{"name": "Alice"})
	if member["id"] == nil {
		t.Fatal("expected member id")
	}
	id := member["id"]

	postJSON(t, apiURL("/contributions"), map[string]any{
		"member_id": id,
		"amount":    50.00,
	})
	postJSON(t, apiURL("/contributions"), map[string]any{
		"member_id": id,
		"amount":    25.50,
	})

	summary := getMap(t, apiURL("/summary"))
	members := summary["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	m := members[0].(map[string]any)
	if m["total"].(float64) != 75.50 {
		t.Errorf("expected total 75.50, got %.2f", m["total"].(float64))
	}

	groupTotal := summary["group_total"].(float64)
	if groupTotal != 75.50 {
		t.Errorf("expected group_total 75.50, got %.2f", groupTotal)
	}
}

func TestFunctionalBadInput(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"empty name", "POST", "/members", map[string]any{"name": ""}, http.StatusBadRequest},
		{"missing name", "POST", "/members", map[string]any{}, http.StatusBadRequest},
		{"zero amount", "POST", "/contributions", map[string]any{"member_id": 1, "amount": 0}, http.StatusBadRequest},
		{"negative amount", "POST", "/contributions", map[string]any{"member_id": 1, "amount": -10}, http.StatusBadRequest},
		{"unknown member", "POST", "/contributions", map[string]any{"member_id": 99999, "amount": 50}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest(tt.method, apiURL(tt.path), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestFunctionalPersistence(t *testing.T) {
	// Verify data survives across requests (same app, same SQLite file on volume)
	resp := getList(t, apiURL("/members"))
	if len(resp) == 0 {
		m := postJSON(t, apiURL("/members"), map[string]any{"name": "PersistTest"})
		id := m["id"]
		postJSON(t, apiURL("/contributions"), map[string]any{
			"member_id": id, "amount": 42.00,
		})

		resp = getList(t, apiURL("/members"))
		if len(resp) == 0 {
			t.Fatal("expected members after insert")
		}
	}

	// Verify summary works
	getMap(t, apiURL("/summary"))
}

func postJSON(t *testing.T, url string, body any) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func getMap(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

func getList(t *testing.T, url string) []any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	var result []any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

func TestMain(m *testing.M) {
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "App not running at %s. Start with: docker compose up app\n", baseURL)
		os.Exit(1)
	}
	resp.Body.Close()

	os.Exit(m.Run())
}
