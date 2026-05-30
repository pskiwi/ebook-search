package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/pskiwi/ebook-search/internal/importer"
	_ "github.com/lib/pq"
)

// looksLikeAuthor catches "Last, First" or plain "Lastname Firstname" patterns.
var reAuthor = regexp.MustCompile(`^[A-ZÄÖÜ][a-zäöüß]+,\s|^[A-ZÄÖÜ][a-zäöüß]+ [A-ZÄÖÜ][a-zäöüß]+$`)

func looksLikeAuthor(s string) bool { return reAuthor.MatchString(s) }

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	ctx := context.Background()
	fixed, skipped := 0, 0

	// Step 1: fix books with garbage or overly-short titles using filename extraction.
	// This also recovers books wrongly set to their series directory name.
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, file_path FROM books
		WHERE title ~ '^[\w\-]+ \d+$'
		   OR title ~ '^[\w\-]+-[\w\-]+ \d+$'
		   OR lower(title) ~ '^layout \d+$'
		   OR title = 'job' OR title = '.'
		   OR (length(title) <= 12 AND file_path ~ ' - .+ - ')
		ORDER BY title`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}

	type entry struct {
		id       int64
		title    string
		filePath string
	}
	var books []entry
	for rows.Next() {
		var b entry
		if err := rows.Scan(&b.id, &b.title, &b.filePath); err != nil {
			log.Fatalf("scan: %v", err)
		}
		books = append(books, b)
	}
	rows.Close()

	log.Printf("%d Bücher zum Prüfen gefunden", len(books))

	for _, b := range books {
		// prefer filename extraction (gets subtitle from "Author - Series NN - Subtitle.ext")
		better := importer.TitleFromFilename(b.filePath)
		if better == "" || looksLikeAuthor(better) {
			better = importer.TitleFromPath(b.filePath)
		}
		if better == b.title || better == "" || looksLikeAuthor(better) {
			skipped++
			continue
		}
		// only replace if longer (has more info) or current title is clearly garbage
		currentIsGarbage := b.title == "." || b.title == "job"
		if !currentIsGarbage && len(better) <= len(b.title) {
			skipped++
			continue
		}
		_, err := db.ExecContext(ctx, `UPDATE books SET title = $1 WHERE id = $2`, better, b.id)
		if err != nil {
			log.Printf("WARN  update %d: %v", b.id, err)
			continue
		}
		fmt.Printf("OK  %q → %q\n", b.title, better)
		fixed++
	}

	log.Printf("Fertig: %d gefixt, %d unverändert", fixed, skipped)
}
