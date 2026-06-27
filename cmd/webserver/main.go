// Package main runs the webserver service, which serves a static HTML site, creates a DB table and inserts page visits into it.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDatabase(dbPath string) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL,
			is_page_view BOOLEAN NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	if err != nil {
		return err
	}
	return nil
}

// Middleware that inserts each request into the requests table
func loggingMiddleware(next http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		isPageView := ext == "" || ext == ".html"

		_, err := db.Exec(`INSERT INTO requests (path, is_page_view) VALUES (?, ?)`, r.URL.Path, isPageView)
		if err != nil {
			log.Println("failed to insert request:", err)
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := initDatabase("data/portfolio.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON"); err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", loggingMiddleware(fileServer, db))
	log.Println("Starting server on :8080")
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
