// Package main runs the metrics service, which polls the webserver for uptime and exposes aggregate traffic stats over HTTP.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func openDatabase(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	return nil
}

// Handler for returning page stats (eg. page visits)
func statsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM visits").Scan(&count)
		if err != nil {
			http.Error(w, "Failed to load stats", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"total_requests": %d}`, count); err != nil {
			log.Println("failed to write response:", err)
		}
	}
}

func main() {
	if err := openDatabase("data/portfolio.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON"); err != nil {
		log.Fatal(err)
	}
	http.Handle("/stats", statsHandler(db))
	log.Println("Starting server on :8081")
	srv := &http.Server{
		Addr:         ":8081",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
