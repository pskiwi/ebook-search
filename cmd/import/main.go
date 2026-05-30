package main

import (
	"context"
	"flag"
	"log"
	"os"

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

	imp := importer.New(pool, *baseDir)
	if err := imp.Run(context.Background(), *ebookDir); err != nil {
		log.Fatalf("import: %v", err)
	}
}
