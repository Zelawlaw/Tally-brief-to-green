//go:build functional

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

const baseURL = "http://localhost:8080"

func apiURL(path string) string {
	return baseURL + path
}

func TestFunctionalCreateMemberAndContribute(t *testing.T) {
	member := postJSON(t, apiURL("/members"), map[string]any{"name": "FunctionalTestMember"})
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
	if len(members) == 0 {
		t.Fatal("expected at least 1 member")
	}

	// Find our member in the summary
	for _, memberIface := range members {
		m := memberIface.(map[string]any)
		if fmt.Sprint(m["id"]) == fmt.Sprint(id) {
			if m["total"].(float64) != 75.50 {
				t.Errorf("expected total 75.50, got %.2f", m["total"].(float64))
			}
			return
		}
	}
	t.Errorf("member id %v not found in summary", id)
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

func TestFunctionalContributionsPage(t *testing.T) {
	m := postJSON(t, apiURL("/members"), map[string]any{"name": "ContribPageTest"})
	id := fmt.Sprint(m["id"])

	html := getHTML(t, baseURL+"/contributions")
	if !strings.Contains(html, "ContribPageTest") {
		t.Error("contributions page should contain member names in the dropdown")
	}
	if !strings.Contains(html, id) {
		t.Error("contributions page should contain member IDs in option values")
	}
}

func TestFunctionalSummaryPage(t *testing.T) {
	m := postJSON(t, apiURL("/members"), map[string]any{"name": "SummaryPageTest"})
	id := m["id"]

	postJSON(t, apiURL("/contributions"), map[string]any{
		"member_id": id, "amount": 100.00,
	})

	html := getHTML(t, baseURL+"/summary-page")
	if !strings.Contains(html, "SummaryPageTest") {
		t.Error("summary page should contain the member's name")
	}
	if !strings.Contains(html, "100.00") {
		t.Error("summary page should contain the contribution total")
	}
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

func TestFunctionalHTMLMembersPageShowsData(t *testing.T) {
	// Create a member via JSON API
	postJSON(t, apiURL("/members"), map[string]any{"name": "HTMLTestMember"})

	// Fetch the HTML members page
	html := getHTML(t, baseURL+"/")

	if !strings.Contains(html, "HTMLTestMember") {
		t.Error("members page should contain the created member's name")
	}
}

func TestFunctionalStatementPage(t *testing.T) {
	m := postJSON(t, apiURL("/members"), map[string]any{"name": "StatementTest"})
	id := m["id"]

	postJSON(t, apiURL("/contributions"), map[string]any{
		"member_id": id, "amount": 42.00, "description": "test payment",
	})

	// Check JSON statement API
	entries := getList(t, apiURL(fmt.Sprintf("/members/%s/statement-json", fmt.Sprint(id))))
	if len(entries) == 0 {
		t.Fatal("expected at least 1 statement entry")
	}

	// Check HTML statement page
	html := getHTML(t, baseURL+fmt.Sprintf("/members/%s/statement", fmt.Sprint(id)))
	if !strings.Contains(html, "StatementTest") {
		t.Error("statement page should contain the member's name")
	}
	if !strings.Contains(html, "42.00") {
		t.Error("statement page should contain the contribution amount")
	}
}

func getHTML(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
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
