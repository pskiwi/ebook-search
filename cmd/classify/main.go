package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/pskiwi/ebook-search/internal/classifier"
	"github.com/pskiwi/ebook-search/internal/db"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	cl := classifier.New(ollamaURL)

	log.Printf("Pulling classification model (llama3.2)…")
	if err := cl.EnsureModel(ctx); err != nil {
		log.Fatalf("pull model: %v", err)
	}

	// Build set of already-clean genres to skip
	cleanGenres := make(map[string]struct{}, len(classifier.ValidGenres))
	for _, g := range classifier.ValidGenres {
		cleanGenres[strings.ToLower(g)] = struct{}{}
	}

	// Step 1: normalize existing messy genres
	distinctGenres, err := db.ListDistinctGenres(ctx, pool)
	if err != nil {
		log.Fatalf("fetch distinct genres: %v", err)
	}
	log.Printf("Step 1: %d distinct Genres normalisieren…", len(distinctGenres))

	for _, g := range distinctGenres {
		if _, clean := cleanGenres[strings.ToLower(g)]; clean {
			continue
		}
		canonical, err := cl.NormalizeGenre(ctx, g)
		if err != nil {
			log.Printf("WARN  %q: %v", g, err)
			continue
		}
		n, err := db.NormalizeGenre(ctx, pool, g, canonical)
		if err != nil {
			log.Printf("WARN  update %q: %v", g, err)
			continue
		}
		log.Printf("OK    %q → %s (%d Bücher)", g, canonical, n)
	}

	// Step 2: classify books still without genre
	books, err := db.BooksWithoutGenre(ctx, pool)
	if err != nil {
		log.Fatalf("fetch books: %v", err)
	}
	log.Printf("Step 2: %d Bücher ohne Genre klassifizieren…", len(books))

	for _, b := range books {
		genre, err := cl.Classify(ctx, b.Title, b.Author)
		if err != nil {
			log.Printf("WARN  %q: %v", b.Title, err)
			continue
		}
		if err := db.UpdateGenre(ctx, pool, b.ID, genre); err != nil {
			log.Printf("WARN  update %q: %v", b.Title, err)
			continue
		}
		log.Printf("OK    %q → %s", b.Title, genre)
	}

	// Step 3: reclassify "Liebesroman" — Ollama decides Liebesroman vs. Schmonzette
	liebesromane, err := db.BooksWithGenre(ctx, pool, "Liebesroman")
	if err != nil {
		log.Fatalf("fetch liebesromane: %v", err)
	}
	log.Printf("Step 3: %d Liebesromane auf Schmonzette prüfen…", len(liebesromane))

	for _, b := range liebesromane {
		genre, err := cl.Classify(ctx, b.Title, b.Author)
		if err != nil {
			log.Printf("WARN  %q: %v", b.Title, err)
			continue
		}
		if genre == "Schmonzette" {
			if err := db.UpdateGenre(ctx, pool, b.ID, genre); err != nil {
				log.Printf("WARN  update %q: %v", b.Title, err)
				continue
			}
			log.Printf("OK    %q → Schmonzette", b.Title)
		}
	}
}
