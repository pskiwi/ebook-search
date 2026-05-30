package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const googleBooksAPI = "https://www.googleapis.com/books/v1/volumes"

type volumeInfo struct {
	Description string `json:"description"`
}

type volume struct {
	VolumeInfo volumeInfo `json:"volumeInfo"`
}

type searchResult struct {
	Items []volume `json:"items"`
}

func fetchDescription(client *http.Client, title, author string) (string, error) {
	q := fmt.Sprintf("intitle:%q", title)
	if author != "" {
		q += fmt.Sprintf("+inauthor:%q", author)
	}
	u := googleBooksAPI + "?q=" + url.QueryEscape(q) + "&maxResults=1&fields=items/volumeInfo/description"

	resp, err := client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var result searchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Items) == 0 {
		return "", nil
	}
	return strings.TrimSpace(result.Items[0].VolumeInfo.Description), nil
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	genre := os.Getenv("GENRE")
	if genre == "" {
		genre = "Schmonzette"
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
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, '') FROM books
		WHERE genre = $1 AND (description IS NULL OR description = '')
		ORDER BY title`, genre)
	if err != nil {
		log.Fatalf("query: %v", err)
	}

	type book struct {
		id     int64
		title  string
		author string
	}
	var books []book
	for rows.Next() {
		var b book
		if err := rows.Scan(&b.id, &b.title, &b.author); err != nil {
			log.Fatalf("scan: %v", err)
		}
		books = append(books, b)
	}
	rows.Close()

	log.Printf("%d %s-Bücher ohne Beschreibung", len(books), genre)

	client := &http.Client{Timeout: 10 * time.Second}
	found, notFound, failed := 0, 0, 0

	for i, b := range books {
		desc, err := fetchDescription(client, b.title, b.author)
		if err != nil {
			if strings.Contains(err.Error(), "rate limited") {
				log.Printf("Rate limit erreicht, warte 60s…")
				time.Sleep(60 * time.Second)
				desc, err = fetchDescription(client, b.title, b.author)
			}
			if err != nil {
				log.Printf("WARN  [%d/%d] %q: %v", i+1, len(books), b.title, err)
				failed++
				continue
			}
		}
		if desc == "" {
			notFound++
			log.Printf("MISS  [%d/%d] %q", i+1, len(books), b.title)
			continue
		}
		_, err = db.ExecContext(ctx, `UPDATE books SET description = $1 WHERE id = $2`, desc, b.id)
		if err != nil {
			log.Printf("WARN  update %q: %v", b.title, err)
			failed++
			continue
		}
		fmt.Printf("OK    [%d/%d] %q\n", i+1, len(books), b.title)
		found++

		// ~1 req/sec to stay within Google's unauthenticated limit
		time.Sleep(time.Second)
	}

	log.Printf("Fertig: %d mit Beschreibung, %d nicht gefunden, %d fehler", found, notFound, failed)
}
