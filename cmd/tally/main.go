package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"tally/internal/handler"
	"tally/internal/store"
)

func main() {
	dbPath := os.Getenv("TALLY_DB_PATH")
	if dbPath == "" {
		dbPath = "tally.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?cache=shared&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	s, err := store.New(db)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() { _ = s.Close() }()

	h, err := handler.New(s, "web/templates")
	if err != nil {
		log.Fatalf("handler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":8080"
	log.Printf("Tally listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // crude app, timeouts not needed for localhost-only service
		log.Fatalf("server: %v", err)
	}
}
