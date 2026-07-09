// Package main runs the metrics service, which polls the webserver for uptime and exposes aggregate traffic stats over HTTP.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
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

// Handler for returning aggregate stats: total requests, total page visits, and uptime percentage.
func statsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var totalRequests, totalVisits int
		err := db.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_page_view THEN 1 ELSE 0 END), 0) FROM requests`,
		).Scan(&totalRequests, &totalVisits)
		if err != nil {
			http.Error(w, "Failed to load requests", http.StatusInternalServerError)
			return
		}

		// Uptime is windowed to the current calendar week (Monday start) so a bad
		// week doesn't drag down future weeks. The denominator is the number of
		// hourly pings expected so far this week (not just rows actually recorded),
		// so a gap where the server itself was down - and no ping could even be
		// attempted - counts against uptime instead of silently vanishing.
		var successfulChecks, expectedPings int
		err = db.QueryRow(`
			SELECT
				COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0),
				CAST((julianday('now') - julianday(date('now', 'weekday 0', '-6 days'))) * 24 AS INTEGER)
			FROM health_checks
			WHERE timestamp >= date('now', 'weekday 0', '-6 days')
		`).Scan(&successfulChecks, &expectedPings)
		if err != nil {
			http.Error(w, "Failed to load uptime", http.StatusInternalServerError)
			return
		}

		uptimePercent := 100.0
		if expectedPings > 0 {
			uptimePercent = float64(successfulChecks) / float64(expectedPings) * 100
			if uptimePercent > 100 {
				uptimePercent = 100
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"total_requests": %d, "total_visits": %d, "uptime_percent": %.2f}`, totalRequests, totalVisits, uptimePercent); err != nil {
			log.Println("failed to write response:", err)
		}
	}
}

type dailyRequestStat struct {
	Date          string `json:"date"`
	TotalRequests int    `json:"total_requests"`
	TotalVisits   int    `json:"total_visits"`
}

// Handler for returning daily request and visit counts for the last 7 days.
func weeklyRequestsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT date(timestamp),
				COUNT(*),
				COALESCE(SUM(CASE WHEN is_page_view THEN 1 ELSE 0 END), 0)
			FROM requests
			WHERE timestamp >= date('now', '-7 days')
			GROUP BY date(timestamp)
			ORDER BY date(timestamp) ASC
		`)
		if err != nil {
			http.Error(w, "Failed to load weekly requests", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		stats := []dailyRequestStat{}
		for rows.Next() {
			var s dailyRequestStat
			if err := rows.Scan(&s.Date, &s.TotalRequests, &s.TotalVisits); err != nil {
				http.Error(w, "Failed to scan weekly requests", http.StatusInternalServerError)
				return
			}
			stats = append(stats, s)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
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
	http.Handle("/api/stats", statsHandler(db))
	http.Handle("/api/requests/weekly", weeklyRequestsHandler(db))
	log.Println("Starting server on :8081")
	srv := &http.Server{
		Addr:         ":8081",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go runHealthChecks(db, "http://localhost:8080/", 1*time.Hour)
	log.Fatal(srv.ListenAndServe())
}
