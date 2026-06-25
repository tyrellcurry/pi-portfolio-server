package main

import (
	"log"
	"net/http"
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path, r.Header)
		next.ServeHTTP(w, r)
	})
}

func main() {
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", loggingMiddleware(fileServer))
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
