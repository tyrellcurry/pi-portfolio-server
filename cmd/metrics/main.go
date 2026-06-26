// Package main runs the metrics service, which polls the webserver for uptime and exposes aggregate traffic stats over HTTP.
package main

import (
	"context"
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
	_, err = db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS health_checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			success BOOLEAN NOT NULL,
			response_time_ms INTEGER,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	)
	if err != nil {
		return err
	}
	return nil
}

// Handler for returning page stats (eg. page visits)
func statsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count, total, successes int
		err := db.QueryRow("SELECT COUNT(*) FROM visits").Scan(&count)
		if err != nil {
			http.Error(w, "Failed to load visits", http.StatusInternalServerError)
			return
		}
		err = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) FROM health_checks`).Scan(&total, &successes)
		if err != nil {
			http.Error(w, "Failed to load uptime", http.StatusInternalServerError)
			return
		}

		var uptimePercent float64
		if total > 0 {
			uptimePercent = float64(successes) / float64(total) * 100
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"total_requests": %d, "uptime_percent": %.2f}`, count, uptimePercent); err != nil {
			log.Println("failed to write response:", err)
		}
	}
}

// checkHealth records a single uptime probe; success requires a 200 response.
func checkHealth(db *sql.DB, url string) {
	start := time.Now()
	resp, err := http.Get(url)
	elapsed := time.Since(start).Milliseconds()

	success := err == nil && resp.StatusCode == http.StatusOK
	if resp != nil {
		_ = resp.Body.Close()
	}

	_, dbErr := db.Exec(
		`INSERT INTO health_checks (success, response_time_ms) VALUES (?, ?)`,
		success, elapsed,
	)
	if dbErr != nil {
		log.Println("failed to record health check:", dbErr)
	}
}

// runHealthChecks blocks, probing targetURL on each tick — call with go.
func runHealthChecks(db *sql.DB, targetURL string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		checkHealth(db, targetURL)
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

	go runHealthChecks(db, "http://localhost:8080/", 1*time.Minute)
	log.Fatal(srv.ListenAndServe())
}
