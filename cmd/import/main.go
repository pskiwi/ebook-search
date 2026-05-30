package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/pskiwi/ebook-search/internal/db"
	"github.com/pskiwi/ebook-search/internal/embeddings"
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
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	embed := embeddings.New(ollamaURL)

	ctx := context.Background()
	log.Println("pulling nomic-embed-text...")
	if err := embed.EnsureModel(ctx); err != nil {
		log.Fatalf("ensure model: %v", err)
	}

	imp := importer.New(pool, embed, *baseDir)
	if err := imp.Run(ctx, *ebookDir); err != nil {
		log.Fatalf("import: %v", err)
	}
}
