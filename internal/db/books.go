package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Book struct {
	ID          int64
	Title       string
	Author      string
	Genre       string
	FilePath    string
	Description string
}


func ListBooks(ctx context.Context, db *sql.DB, genre string, offset int) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), COALESCE(genre, ''), file_path
		FROM books
		WHERE ($1 = '' OR genre ILIKE '%' || $1 || '%')
		ORDER BY title
		LIMIT 101 OFFSET $2`, genre, offset)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func SearchBooks(ctx context.Context, db *sql.DB, query, genre string, offset int) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), COALESCE(genre, ''), file_path
		FROM books
		WHERE (title ILIKE $1 OR author ILIKE $1)
		  AND ($2 = '' OR genre ILIKE '%' || $2 || '%')
		ORDER BY title
		LIMIT 101 OFFSET $3`,
		"%"+query+"%", genre, offset)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func CountBooks(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM books`).Scan(&n)
	return n, err
}

func ListGenres(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT genre FROM books
		WHERE genre IS NOT NULL AND genre != ''
		GROUP BY genre
		HAVING COUNT(*) >= 50
		ORDER BY genre`)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	defer rows.Close()
	var genres []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

func BooksWithGenre(ctx context.Context, db *sql.DB, genre string) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), COALESCE(genre, ''), file_path
		FROM books WHERE genre = $1 ORDER BY title`, genre)
	if err != nil {
		return nil, fmt.Errorf("books with genre: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func BooksWithoutGenre(ctx context.Context, db *sql.DB) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(author, ''), COALESCE(genre, ''), file_path
		FROM books
		WHERE genre IS NULL OR genre = ''
		ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("books without genre: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func UpdateGenre(ctx context.Context, db *sql.DB, id int64, genre string) error {
	_, err := db.ExecContext(ctx, `UPDATE books SET genre = $1 WHERE id = $2`, genre, id)
	return err
}

func ListDistinctGenres(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT genre FROM books
		WHERE genre IS NOT NULL AND genre != ''
		GROUP BY genre ORDER BY genre`)
	if err != nil {
		return nil, fmt.Errorf("list distinct genres: %w", err)
	}
	defer rows.Close()
	var genres []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

func NormalizeGenre(ctx context.Context, db *sql.DB, oldGenre, newGenre string) (int64, error) {
	res, err := db.ExecContext(ctx, `UPDATE books SET genre = $1 WHERE genre = $2`, newGenre, oldGenre)
	if err != nil {
		return 0, fmt.Errorf("normalize genre: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func GetBook(ctx context.Context, db *sql.DB, id int64) (Book, error) {
	var b Book
	err := db.QueryRowContext(ctx, `
		SELECT id, title, COALESCE(author,''), COALESCE(genre,''), file_path, COALESCE(description,'')
		FROM books WHERE id = $1`, id).
		Scan(&b.ID, &b.Title, &b.Author, &b.Genre, &b.FilePath, &b.Description)
	return b, err
}

func IsInReadingList(ctx context.Context, db *sql.DB, bookID int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reading_list WHERE book_id = $1)`, bookID).Scan(&exists)
	return exists, err
}

func ToggleReadingList(ctx context.Context, db *sql.DB, bookID int64) (inList bool, err error) {
	in, err := IsInReadingList(ctx, db, bookID)
	if err != nil {
		return false, err
	}
	if in {
		_, err = db.ExecContext(ctx, `DELETE FROM reading_list WHERE book_id = $1`, bookID)
		return false, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO reading_list (book_id) VALUES ($1) ON CONFLICT DO NOTHING`, bookID)
	return true, err
}

func GetReadingList(ctx context.Context, db *sql.DB) ([]Book, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT b.id, b.title, COALESCE(b.author,''), COALESCE(b.genre,''), b.file_path
		FROM books b
		JOIN reading_list r ON r.book_id = b.id
		ORDER BY r.added_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("get reading list: %w", err)
	}
	defer rows.Close()
	return scanBooks(rows)
}

func CountReadingList(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reading_list`).Scan(&n)
	return n, err
}

func scanBooks(rows *sql.Rows) ([]Book, error) {
	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Genre, &b.FilePath); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
