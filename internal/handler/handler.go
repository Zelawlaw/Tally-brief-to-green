package handler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"tally/internal/store"
)

const (
	hdrContentType  = "Content-Type"
	contentTypeJSON = "application/json; charset=utf-8"
	contentTypeHTML = "text/html; charset=utf-8"
)

// Store is the interface the handler needs from the storage layer.
type Store interface {
	CreateMember(name string) (*store.Member, error)
	GetMembers() ([]store.Member, error)
	GetMember(id int64) (*store.Member, error)
	CreateContribution(memberID int64, amount float64, description string) (*store.Contribution, error)
	GetSummary() (*store.Summary, error)
	GetStatement(memberID int64) ([]store.StatementEntry, error)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	Store     Store
	templates map[string]*template.Template // page name → parsed template
}

const (
	pageMembers       = "members.html"
	pageContributions = "contributions.html"
	pageSummary       = "summary.html"
	pageStatement     = "statement.html"
)

// New creates a Handler with parsed templates.
// Each page template is parsed separately with base.html so "content" blocks don't clash.
func New(s Store, templatesDir string) (*Handler, error) {
	pages := []string{pageMembers, pageContributions, pageSummary, pageStatement}
	tmpls := make(map[string]*template.Template)

	for _, page := range pages {
		tmpl, err := template.ParseFiles(
			filepath.Join(templatesDir, "base.html"),
			filepath.Join(templatesDir, page),
		)
		if err != nil {
			return nil, err
		}
		tmpls[page] = tmpl
	}

	return &Handler{Store: s, templates: tmpls}, nil
}

// RegisterRoutes sets up all routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /members", h.CreateMember)
	mux.HandleFunc("GET /members", h.GetMembers)
	mux.HandleFunc("POST /contributions", h.CreateContribution)
	mux.HandleFunc("GET /summary", h.GetSummary)
	mux.HandleFunc("GET /", h.PageMembers)
	mux.HandleFunc("GET /contributions", h.PageContributions)
	mux.HandleFunc("GET /summary-page", h.PageSummary)
	mux.HandleFunc("GET /members/{id}/statement", h.PageStatement)
	mux.HandleFunc("GET /members/{id}/statement-json", h.MemberStatementJSON)
}

// isJSON returns true if the request Content-Type is application/json.
func isJSON(r *http.Request) bool {
	ct := r.Header.Get(hdrContentType)
	return strings.HasPrefix(ct, "application/json")
}

// --- POST /members ---

func (h *Handler) CreateMember(w http.ResponseWriter, r *http.Request) {
	var name string

	if isJSON(r) {
		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		name = req.Name
	} else {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form data")
			return
		}
		name = r.FormValue("name")
	}

	m, err := h.Store.CreateMember(name)
	if err != nil {
		if isJSON(r) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	if isJSON(r) {
		writeJSON(w, http.StatusCreated, m)
		return
	}

	members, _ := h.Store.GetMembers()
	w.Header().Set(hdrContentType, contentTypeHTML)
	_ = h.templates[pageMembers].ExecuteTemplate(w, "members-table", members)
}

// --- GET /members ---

func (h *Handler) GetMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.Store.GetMembers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

// --- POST /contributions ---

func (h *Handler) CreateContribution(w http.ResponseWriter, r *http.Request) {
	var memberID int64
	var amount float64
	var description string

	if isJSON(r) {
		var req struct {
			MemberID    int64   `json:"member_id"`
			Amount      float64 `json:"amount"`
			Description string  `json:"description"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		memberID = req.MemberID
		amount = req.Amount
		description = req.Description
	} else {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form data")
			return
		}
		var err error
		memberID, err = strconv.ParseInt(r.FormValue("member_id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid member_id")
			return
		}
		amount, err = strconv.ParseFloat(r.FormValue("amount"), 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid amount")
			return
		}
		description = r.FormValue("description")
	}

	c, err := h.Store.CreateContribution(memberID, amount, description)
	if err != nil {
		if isJSON(r) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	if isJSON(r) {
		writeJSON(w, http.StatusCreated, c)
		return
	}

	w.Header().Set(hdrContentType, contentTypeHTML)
	//nolint:gosec // c.Amount is a validated, rounded float64 — cannot contain executable content
	_, _ = w.Write([]byte("<p>Contribution added: " + formatCurrency(c.Amount) + "</p>"))
}

// --- GET /summary ---

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	s, err := h.Store.GetSummary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// --- HTML page handlers ---

func (h *Handler) PageMembers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	members, err := h.Store.GetMembers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, pageMembers, map[string]any{"Members": members})
}

func (h *Handler) PageContributions(w http.ResponseWriter, r *http.Request) {
	members, err := h.Store.GetMembers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, pageContributions, map[string]any{"Members": members})
}

func (h *Handler) PageSummary(w http.ResponseWriter, r *http.Request) {
	s, err := h.Store.GetSummary()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, pageSummary, map[string]any{"Summary": s})
}

func (h *Handler) MemberStatementJSON(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	m, err := h.Store.GetMember(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	entries, err := h.Store.GetStatement(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) PageStatement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid member id", http.StatusBadRequest)
		return
	}

	m, err := h.Store.GetMember(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.NotFound(w, r)
		return
	}

	entries, err := h.Store.GetStatement(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, pageStatement, map[string]any{
		"Member":  m,
		"Entries": entries,
	})
}

// --- helpers ---

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set(hdrContentType, contentTypeHTML)
	_ = h.templates[page].ExecuteTemplate(w, page, data)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(hdrContentType, contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func formatCurrency(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// ParseID extracts an int64 ID from a path segment like /members/3/statement.
func ParseID(path, prefix string) (int64, error) {
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.TrimSuffix(idStr, "/statement")
	idStr = strings.TrimSuffix(idStr, "/")
	return strconv.ParseInt(idStr, 10, 64)
}
