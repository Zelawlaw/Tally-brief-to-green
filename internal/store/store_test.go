package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	s, err := New(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestNewStoreError(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	_ = db.Close()
	_, err := New(db)
	if err == nil {
		t.Error("expected error with closed db")
	}
}

func TestCreateMember(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "Alice", false},
		{"empty name", "", true},
		{"whitespace only", "   ", true},
		{"trimmed", "  Bob  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := s.CreateMember(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.ID == 0 {
				t.Error("expected non-zero ID")
			}
			if m.CreatedAt == "" {
				t.Error("expected non-empty created_at")
			}
		})
	}
}

func TestGetMembers(t *testing.T) {
	s := newTestStore(t)

	members, err := s.GetMembers()
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("expected 0 members, got %d", len(members))
	}

	_, _ = s.CreateMember("Alice")
	_, _ = s.CreateMember("Bob")

	members, err = s.GetMembers()
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", members[0].Name)
	}
	if members[1].Name != "Bob" {
		t.Errorf("expected Bob, got %s", members[1].Name)
	}
}

func TestGetMember(t *testing.T) {
	s := newTestStore(t)

	m, err := s.GetMember(999)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Error("expected nil for non-existent member")
	}

	created, _ := s.CreateMember("Alice")
	m, err = s.GetMember(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected member, got nil")
		return
	}
	if m.Name != "Alice" {
		t.Errorf("expected Alice, got %s", m.Name)
	}
}

func TestCreateContribution(t *testing.T) {
	s := newTestStore(t)
	member, _ := s.CreateMember("Alice")

	tests := []struct {
		name        string
		memberID    int64
		amount      float64
		description string
		wantErr     bool
	}{
		{"valid contribution", member.ID, 50.00, "rent", false},
		{"zero amount", member.ID, 0, "nope", true},
		{"negative amount", member.ID, -10, "nope", true},
		{"unknown member", 99999, 50, "nope", true},
		{"no description", member.ID, 25.00, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := s.CreateContribution(tt.memberID, tt.amount, tt.description)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.ID == 0 {
				t.Error("expected non-zero ID")
			}
			if c.CreatedAt == "" {
				t.Error("expected non-empty created_at")
			}
			if c.Amount != tt.amount {
				t.Errorf("expected amount %.2f, got %.2f", tt.amount, c.Amount)
			}
		})
	}
}

func TestGetSummary(t *testing.T) {
	s := newTestStore(t)

	alice, _ := s.CreateMember("Alice")
	bob, _ := s.CreateMember("Bob")

	_, _ = s.CreateContribution(alice.ID, 100.00, "first")
	_, _ = s.CreateContribution(alice.ID, 50.00, "second")
	_, _ = s.CreateContribution(bob.ID, 75.00, "bob's")

	summary, err := s.GetSummary()
	if err != nil {
		t.Fatal(err)
	}

	if len(summary.Members) != 2 {
		t.Fatalf("expected 2 members in summary, got %d", len(summary.Members))
	}
	if summary.Members[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", summary.Members[0].Name)
	}
	if summary.Members[0].Total != 150.00 {
		t.Errorf("expected 150.00, got %.2f", summary.Members[0].Total)
	}
	if summary.Members[1].Name != "Bob" {
		t.Errorf("expected Bob, got %s", summary.Members[1].Name)
	}
	if summary.Members[1].Total != 75.00 {
		t.Errorf("expected 75.00, got %.2f", summary.Members[1].Total)
	}
	if summary.GroupTotal != 225.00 {
		t.Errorf("expected 225.00 group total, got %.2f", summary.GroupTotal)
	}
}

func TestSummaryEmpty(t *testing.T) {
	s := newTestStore(t)
	summary, err := s.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Members) != 0 {
		t.Errorf("expected empty members, got %d", len(summary.Members))
	}
	if summary.GroupTotal != 0 {
		t.Errorf("expected 0 group total, got %.2f", summary.GroupTotal)
	}
}

func TestSummaryMemberNoContributions(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateMember("Alice")
	summary, err := s.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Members[0].Total != 0 {
		t.Errorf("expected 0 total for member with no contributions, got %.2f", summary.Members[0].Total)
	}
}

// --- Error path tests: close db then try operations ---

func TestClosedStoreErrors(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	s, _ := New(db)
	_, _ = s.CreateMember("Alice")
	_ = s.Close()

	if _, err := s.CreateMember("Bob"); err == nil {
		t.Error("expected error after close")
	}
	if _, err := s.GetMembers(); err == nil {
		t.Error("expected error after close")
	}
	if _, err := s.GetMember(1); err == nil {
		t.Error("expected error after close")
	}
	if _, err := s.GetSummary(); err == nil {
		t.Error("expected error after close")
	}
}

func TestGetStatement(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.CreateMember("Alice")
	_, _ = s.CreateContribution(m.ID, 100.00, "first")
	_, _ = s.CreateContribution(m.ID, 50.00, "second")
	_, _ = s.CreateContribution(m.ID, 25.00, "third")

	entries, err := s.GetStatement(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Balance != 100.00 {
		t.Errorf("expected balance 100.00, got %.2f", entries[0].Balance)
	}
	if entries[1].Balance != 150.00 {
		t.Errorf("expected balance 150.00, got %.2f", entries[1].Balance)
	}
	if entries[2].Balance != 175.00 {
		t.Errorf("expected balance 175.00, got %.2f", entries[2].Balance)
	}
}

func TestGetStatementEmpty(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.CreateMember("Alice")
	entries, err := s.GetStatement(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entries == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetStatementClosedStore(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	s, _ := New(db)
	_ = s.Close()
	_, err := s.GetStatement(1)
	if err == nil {
		t.Error("expected error after close")
	}
}

func TestClosedStoreContributionError(t *testing.T) {
	db, _ := sql.Open("sqlite", ":memory:?cache=shared")
	s, _ := New(db)
	_ = s.Close()

	// memberExists on closed DB returns false, which triggers "member not found" error
	_, err := s.CreateContribution(1, 50, "")
	if err == nil {
		t.Error("expected error after close")
	}
}
