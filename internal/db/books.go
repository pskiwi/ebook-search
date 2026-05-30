package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Book struct {
	ID       int64
	Title    string
	Author   string
	FilePath string
}

func ListBooks(ctx context.Context, db *sql.DB) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), file_path
		FROM books
		ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func SearchBooks(ctx context.Context, db *sql.DB, query string) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), file_path
		FROM books
		WHERE title ILIKE $1 OR author ILIKE $1
		ORDER BY title
		LIMIT 100`,
		"%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func scanBooks(rows *sql.Rows) ([]Book, error) {
	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.FilePath); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
