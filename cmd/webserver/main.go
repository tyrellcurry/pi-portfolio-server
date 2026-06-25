package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
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
		`CREATE TABLE IF NOT EXISTS visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	if err != nil {
		return err
	}
	return nil
}

func loggingMiddleware(next http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := db.Exec(`INSERT INTO visits (path) VALUES (?)`, r.URL.Path)
		if err != nil {
			log.Println("failed to insert visit:", err)
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := initDatabase("data/portfolio.db"); err != nil {
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
