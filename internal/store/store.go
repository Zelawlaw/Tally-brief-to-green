package store

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	_ "modernc.org/sqlite"
)

// Member represents a group member.
type Member struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Contribution represents a single contribution by a member.
type Contribution struct {
	ID          int64   `json:"id"`
	MemberID    int64   `json:"member_id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
}

// MemberSummary is a member with their total contributions.
type MemberSummary struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

// Summary is the full group summary.
type Summary struct {
	Members    []MemberSummary `json:"members"`
	GroupTotal float64         `json:"group_total"`
}

// Store provides access to the SQLite database.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS members (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS contributions (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	member_id   INTEGER NOT NULL REFERENCES members(id),
	amount      REAL NOT NULL CHECK (amount > 0),
	description TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// New initializes a Store from an existing sql.DB, running migrations.
// The caller is responsible for opening and closing the database connection.
func New(db *sql.DB) (*Store, error) {
	if _, err := db.Exec("PRAGMA foreign_keys = ON; " + schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateMember inserts a new member.
func (s *Store) CreateMember(name string) (*Member, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	m := &Member{Name: name}
	err := s.db.QueryRow(
		"INSERT INTO members (name) VALUES (?) RETURNING id, created_at",
		name,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetMembers returns all members.
func (s *Store) GetMembers() ([]Member, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM members ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []Member
	for rows.Next() {
		var m Member
		_ = rows.Scan(&m.ID, &m.Name, &m.CreatedAt)
		members = append(members, m)
	}
	if members == nil {
		members = []Member{}
	}
	return members, rows.Err()
}

// GetMember returns a single member by ID.
func (s *Store) GetMember(id int64) (*Member, error) {
	var m Member
	err := s.db.QueryRow(
		"SELECT id, name, created_at FROM members WHERE id = ?", id,
	).Scan(&m.ID, &m.Name, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateContribution inserts a new contribution.
func (s *Store) CreateContribution(memberID int64, amount float64, description string) (*Contribution, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	amount = math.Round(amount*100) / 100

	if !s.memberExists(memberID) {
		return nil, fmt.Errorf("member not found")
	}

	c := &Contribution{MemberID: memberID, Amount: amount, Description: description}
	_ = s.db.QueryRow(
		"INSERT INTO contributions (member_id, amount, description) VALUES (?, ?, ?) RETURNING id, created_at",
		memberID, amount, description,
	).Scan(&c.ID, &c.CreatedAt)
	return c, nil
}

// GetSummary returns per-member totals and the group total.
func (s *Store) GetSummary() (*Summary, error) {
	rows, err := s.db.Query(`
		SELECT m.id, m.name, COALESCE(SUM(c.amount), 0) AS total
		FROM members m
		LEFT JOIN contributions c ON c.member_id = m.id
		GROUP BY m.id
		ORDER BY m.id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []MemberSummary
	var groupTotal float64
	for rows.Next() {
		var ms MemberSummary
		_ = rows.Scan(&ms.ID, &ms.Name, &ms.Total)
		ms.Total = math.Round(ms.Total*100) / 100
		members = append(members, ms)
		groupTotal += ms.Total
	}
	if members == nil {
		members = []MemberSummary{}
	}
	groupTotal = math.Round(groupTotal*100) / 100
	return &Summary{Members: members, GroupTotal: groupTotal}, rows.Err()
}

// StatementEntry is a single contribution row for a member statement.
type StatementEntry struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
	Balance     float64 `json:"balance"`
}

// GetStatement returns a member's contributions in chronological order with running balance.
func (s *Store) GetStatement(memberID int64) ([]StatementEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, amount, description, created_at FROM contributions WHERE member_id = ? ORDER BY created_at, id",
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []StatementEntry
	var balance float64
	for rows.Next() {
		var e StatementEntry
		_ = rows.Scan(&e.ID, &e.Amount, &e.Description, &e.CreatedAt)
		e.Amount = math.Round(e.Amount*100) / 100
		balance += e.Amount
		e.Balance = math.Round(balance*100) / 100
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []StatementEntry{}
	}
	return entries, rows.Err()
}

func (s *Store) memberExists(id int64) bool {
	var exists bool
	_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM members WHERE id = ?)", id).Scan(&exists)
	return exists
}
