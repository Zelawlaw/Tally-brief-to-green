package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"tally/internal/store"
)

// errorStore returns configurable errors for testing error paths.
type errorStore struct {
	errMembers       error
	errMember        error
	errCreateMember  error
	errContrib       error
	errSummary       error
	members          []store.Member
	contributions    []store.Contribution
	nextMemID        int64
	nextConID        int64
}

func (e *errorStore) CreateMember(name string) (*store.Member, error) {
	if e.errCreateMember != nil {
		return nil, e.errCreateMember
	}
	e.nextMemID++
	m := &store.Member{ID: e.nextMemID, Name: name, CreatedAt: "2026-07-04T00:00:00"}
	e.members = append(e.members, *m)
	return m, nil
}

func (e *errorStore) GetMembers() ([]store.Member, error) {
	if e.errMembers != nil {
		return nil, e.errMembers
	}
	if e.members == nil {
		return []store.Member{}, nil
	}
	return e.members, nil
}

func (e *errorStore) GetMember(id int64) (*store.Member, error) {
	if e.errMember != nil {
		return nil, e.errMember
	}
	for _, m := range e.members {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, nil
}

func (e *errorStore) CreateContribution(memberID int64, amount float64, description string) (*store.Contribution, error) {
	if e.errContrib != nil {
		return nil, e.errContrib
	}
	e.nextConID++
	c := &store.Contribution{ID: e.nextConID, MemberID: memberID, Amount: amount, Description: description, CreatedAt: "2026-07-04T00:00:00"}
	e.contributions = append(e.contributions, *c)
	return c, nil
}

func (e *errorStore) GetStatement(memberID int64) ([]store.StatementEntry, error) {
	if e.errSummary != nil {
		return nil, e.errSummary
	}
	var entries []store.StatementEntry
	var balance float64
	for _, c := range e.contributions {
		if c.MemberID == memberID {
			balance += c.Amount
			entries = append(entries, store.StatementEntry{
				ID: c.ID, Amount: c.Amount, Description: c.Description,
				CreatedAt: c.CreatedAt, Balance: balance,
			})
		}
	}
	if entries == nil {
		entries = []store.StatementEntry{}
	}
	return entries, nil
}

func (e *errorStore) GetSummary() (*store.Summary, error) {
	if e.errSummary != nil {
		return nil, e.errSummary
	}
	var ms []store.MemberSummary
	var gt float64
	for _, m := range e.members {
		t := float64(0)
		for _, c := range e.contributions {
			if c.MemberID == m.ID {
				t += c.Amount
			}
		}
		ms = append(ms, store.MemberSummary{ID: m.ID, Name: m.Name, Total: t})
		gt += t
	}
	if ms == nil {
		ms = []store.MemberSummary{}
	}
	return &store.Summary{Members: ms, GroupTotal: gt}, nil
}

func newHandlerWithTemplates(t *testing.T) *Handler {
	t.Helper()
	h, err := New(&errorStore{}, "../../web/templates")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	return h
}

// --- JSON API success tests ---

func TestCreateMemberAPI(t *testing.T) {
	h := newHandlerWithTemplates(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid", `{"name": "Alice"}`, http.StatusCreated},
		{"empty json", `{}`, http.StatusCreated},
		{"invalid json", `not json`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/members", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.CreateMember(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestCreateMemberJSONError(t *testing.T) {
	s := &errorStore{errCreateMember: errors.New("db error")}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("POST", "/members", strings.NewReader(`{"name": "x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateMember(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateMemberForm(t *testing.T) {
	h := newHandlerWithTemplates(t)

	form := strings.NewReader("name=Alice")
	req := httptest.NewRequest("POST", "/members", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateMember(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateMemberFormError(t *testing.T) {
	s := &errorStore{errCreateMember: errors.New("boom")}
	h, _ := New(s, "../../web/templates")

	form := strings.NewReader("name=x")
	req := httptest.NewRequest("POST", "/members", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateMember(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetMembersAPI(t *testing.T) {
	h := newHandlerWithTemplates(t)

	req := httptest.NewRequest("GET", "/members", nil)
	rec := httptest.NewRecorder()
	h.GetMembers(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetMembersError(t *testing.T) {
	s := &errorStore{errMembers: errors.New("db error")}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("GET", "/members", nil)
	rec := httptest.NewRecorder()
	h.GetMembers(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestCreateContributionAPI(t *testing.T) {
	s := &errorStore{}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("POST", "/contributions", strings.NewReader(`{"member_id":1,"amount":50}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestCreateContributionJSONError(t *testing.T) {
	s := &errorStore{errContrib: errors.New("db error")}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("POST", "/contributions", strings.NewReader(`{"member_id":1,"amount":50}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateContributionInvalidJSON(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("POST", "/contributions", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateContributionFormError(t *testing.T) {
	s := &errorStore{errContrib: errors.New("boom")}
	h, _ := New(s, "../../web/templates")

	form := strings.NewReader("member_id=1&amount=50")
	req := httptest.NewRequest("POST", "/contributions", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateContributionForm(t *testing.T) {
	s := &errorStore{}
	h, _ := New(s, "../../web/templates")

	form := strings.NewReader("member_id=1&amount=50&description=rent")
	req := httptest.NewRequest("POST", "/contributions", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateContributionFormBadMemberID(t *testing.T) {
	s := &errorStore{}
	h, _ := New(s, "../../web/templates")

	form := strings.NewReader("member_id=abc&amount=50")
	req := httptest.NewRequest("POST", "/contributions", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateContributionFormBadAmount(t *testing.T) {
	s := &errorStore{}
	h, _ := New(s, "../../web/templates")

	form := strings.NewReader("member_id=1&amount=xyz")
	req := httptest.NewRequest("POST", "/contributions", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetSummaryAPI(t *testing.T) {
	s := &errorStore{
		members:       []store.Member{{ID: 1, Name: "Alice", CreatedAt: "..."}},
		contributions: []store.Contribution{{ID: 1, MemberID: 1, Amount: 50.00}},
	}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("GET", "/summary", nil)
	rec := httptest.NewRecorder()
	h.GetSummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var s2 store.Summary
	_ = json.NewDecoder(rec.Body).Decode(&s2)
	if s2.GroupTotal != 50.00 {
		t.Errorf("expected 50.00, got %.2f", s2.GroupTotal)
	}
}

func TestGetSummaryError(t *testing.T) {
	s := &errorStore{errSummary: errors.New("db error")}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("GET", "/summary", nil)
	rec := httptest.NewRecorder()
	h.GetSummary(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// --- Page handler tests ---

func TestPageMembers(t *testing.T) {
	h := newHandlerWithTemplates(t)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.PageMembers(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPageMembers404(t *testing.T) {
	h := newHandlerWithTemplates(t)
	req := httptest.NewRequest("GET", "/nope", nil)
	rec := httptest.NewRecorder()
	h.PageMembers(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestPageMembersError(t *testing.T) {
	s := &errorStore{errMembers: errors.New("db error")}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.PageMembers(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestPageContributions(t *testing.T) {
	h := newHandlerWithTemplates(t)
	req := httptest.NewRequest("GET", "/contributions", nil)
	rec := httptest.NewRecorder()
	h.PageContributions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPageContributionsError(t *testing.T) {
	s := &errorStore{errMembers: errors.New("db error")}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/contributions", nil)
	rec := httptest.NewRecorder()
	h.PageContributions(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestPageSummary(t *testing.T) {
	h := newHandlerWithTemplates(t)
	req := httptest.NewRequest("GET", "/summary-page", nil)
	rec := httptest.NewRecorder()
	h.PageSummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPageSummaryError(t *testing.T) {
	s := &errorStore{errSummary: errors.New("db error")}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/summary-page", nil)
	rec := httptest.NewRecorder()
	h.PageSummary(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// --- New with invalid template dir ---

func TestNewHandlerInvalidTemplateDir(t *testing.T) {
	_, err := New(&errorStore{}, "/nonexistent/path")
	if err == nil {
		t.Error("expected error for invalid template dir")
	}
}

// --- RegisterRoutes test ---

func TestRegisterRoutes(t *testing.T) {
	s := &errorStore{}
	h, _ := New(s, "../../web/templates")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/", 200},
		{"GET", "/contributions", 200},
		{"GET", "/summary-page", 200},
		{"POST", "/members", 200},
		{"GET", "/members", 200},
		{"GET", "/summary", 200},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.method == "POST" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Body = io.NopCloser(strings.NewReader("name=x"))
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf("expected %d, got %d", tt.status, rec.Code)
			}
		})
	}
}

// Test that non-JSON content type with a JSON body falls through to form parsing.
func TestCreateMemberNonJSONContentType(t *testing.T) {
	req := httptest.NewRequest("POST", "/members", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h, _ := New(&errorStore{}, "../../web/templates")
	h.CreateMember(rec, req)
	// Falls through to form parsing — no "name" field in form → empty name → creates anyway in errorStore
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from form path, got %d", rec.Code)
	}
}

// --- ParseID ---

func TestParseID(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   int64
	}{
		{"/members/3/statement", "/members/", 3},
		{"/members/42/statement", "/members/", 42},
	}
	for _, tt := range tests {
		got, err := ParseID(tt.path, tt.prefix)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got != tt.want {
			t.Errorf("expected %d, got %d", tt.want, got)
		}
	}
}

// --- main_test.go: TestNew creates temporary templates for tests that don't need real ones ---

func TestNewCreatesHandler(t *testing.T) {
	dir := t.TempDir()
	// base.html + all four page templates required by New
	_ = os.WriteFile(dir+"/base.html", []byte("{{define \"base\"}}{{template \"content\" .}}{{end}}"), 0600)
	for _, page := range []string{"members.html", "contributions.html", "summary.html", "statement.html"} {
		_ = os.WriteFile(dir+"/"+page, []byte("{{define \""+page+"\"}}{{template \"base\" .}}{{end}}{{define \"content\"}}test{{end}}"), 0600)
	}
	h, err := New(&errorStore{}, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if h.Store == nil {
		t.Error("expected store")
	}
	if len(h.templates) != 4 {
		t.Errorf("expected 4 templates, got %d", len(h.templates))
	}
}

// errorReader is a reader that always returns an error.
type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

func TestCreateMemberParseFormError(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("POST", "/members", nil)
	req.Body = io.NopCloser(errorReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateMember(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateContributionParseFormError(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("POST", "/contributions", nil)
	req.Body = io.NopCloser(errorReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.CreateContribution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- Statement tests ---

func TestMemberStatementJSON(t *testing.T) {
	s := &errorStore{
		members:       []store.Member{{ID: 1, Name: "Alice", CreatedAt: "..."}},
		contributions: []store.Contribution{{ID: 1, MemberID: 1, Amount: 50.00, CreatedAt: "..."}},
	}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("GET", "/members/1/statement-json", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.MemberStatementJSON(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMemberStatementJSONBadID(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/abc/statement-json", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.MemberStatementJSON(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMemberStatementJSONNotFound(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/99/statement-json", nil)
	req.SetPathValue("id", "99")
	rec := httptest.NewRecorder()
	h.MemberStatementJSON(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMemberStatementJSONGetMemberError(t *testing.T) {
	s := &errorStore{errMember: errors.New("db error")}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/1/statement-json", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.MemberStatementJSON(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestMemberStatementJSONGetStatementError(t *testing.T) {
	s := &errorStore{
		members:    []store.Member{{ID: 1, Name: "Alice", CreatedAt: "..."}},
		errSummary: errors.New("db error"),
	}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/1/statement-json", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.MemberStatementJSON(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestPageStatement(t *testing.T) {
	s := &errorStore{
		members:       []store.Member{{ID: 1, Name: "Alice", CreatedAt: "..."}},
		contributions: []store.Contribution{{ID: 1, MemberID: 1, Amount: 50.00, CreatedAt: "..."}},
	}
	h, _ := New(s, "../../web/templates")

	req := httptest.NewRequest("GET", "/members/1/statement", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.PageStatement(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPageStatementBadID(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/abc/statement", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.PageStatement(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestPageStatementNotFound(t *testing.T) {
	h, _ := New(&errorStore{}, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/99/statement", nil)
	req.SetPathValue("id", "99")
	rec := httptest.NewRecorder()
	h.PageStatement(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestPageStatementGetMemberError(t *testing.T) {
	s := &errorStore{errMember: errors.New("db error")}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/1/statement", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.PageStatement(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestPageStatementGetStatementError(t *testing.T) {
	s := &errorStore{
		members:    []store.Member{{ID: 1, Name: "Alice", CreatedAt: "..."}},
		errSummary: errors.New("db error"),
	}
	h, _ := New(s, "../../web/templates")
	req := httptest.NewRequest("GET", "/members/1/statement", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	h.PageStatement(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestFormatCurrency(t *testing.T) {
	if s := formatCurrency(12.5); s != "12.50" {
		t.Errorf("expected 12.50, got %s", s)
	}
	if s := formatCurrency(0); s != "0.00" {
		t.Errorf("expected 0.00, got %s", s)
	}
}
