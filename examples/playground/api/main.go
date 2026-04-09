package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"api"}`)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pgHost := os.Getenv("POSTGRES_HOST")
		redisHost := os.Getenv("REDIS_HOST")
		fmt.Fprintf(w, `{"service":"api","postgres":"%s","redis":"%s"}`, pgHost, redisHost)
	})

	fmt.Printf("API listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
