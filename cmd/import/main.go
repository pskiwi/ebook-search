package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/pskiwi/ebook-search/internal/classifier"
	"github.com/pskiwi/ebook-search/internal/db"
	"github.com/pskiwi/ebook-search/internal/importer"
)

func main() {
	baseDir := flag.String("base-dir", "/Volumes/Daten/ebooks", "base directory (paths stored relative to this)")
	ebookDir := flag.String("dir", "/Volumes/Daten/ebooks/ebooks", "directory to scan for ebooks")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	imp := importer.New(pool, *baseDir)

	if ollamaURL := os.Getenv("OLLAMA_URL"); ollamaURL != "" {
		cl := classifier.New(ollamaURL)
		log.Printf("Pulling classification model (llama3.2), this may take a while on first run…")
		if err := cl.EnsureModel(ctx); err != nil {
			log.Printf("WARN  could not pull classifier model: %v — skipping genre classification", err)
		} else {
			imp.WithClassifier(cl)
		}
	}

	if err := imp.Run(ctx, *ebookDir); err != nil {
		log.Fatalf("import: %v", err)
	}
}
