package main

import (
	"log"
	"net/http"
	"os"

	"github.com/pskiwi/ebook-search/internal/db"
	"github.com/pskiwi/ebook-search/internal/handler"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	handler.Register(mux, pool, os.Getenv("GOOGLE_BOOKS_API_KEY"))

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
