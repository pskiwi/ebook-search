package importer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pskiwi/ebook-search/internal/embeddings"
	"github.com/pskiwi/ebook-search/internal/parser"
)

type Importer struct {
	db      *sql.DB
	embed   *embeddings.Client
	baseDir string
}

func New(db *sql.DB, embed *embeddings.Client, baseDir string) *Importer {
	return &Importer{db: db, embed: embed, baseDir: baseDir}
}

func (imp *Importer) Run(ctx context.Context, dir string) error {
	var imported, skipped, failed int
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".epub" && ext != ".pdf" {
			return nil
		}
		changed, importErr := imp.importFile(ctx, path)
		switch {
		case importErr != nil:
			log.Printf("FAIL  %s: %v", filepath.Base(path), importErr)
			failed++
		case !changed:
			skipped++
		default:
			imported++
		}
		return nil
	})
	log.Printf("done: %d imported, %d unchanged, %d failed", imported, skipped, failed)
	return err
}

// importFile returns (true, nil) if imported, (false, nil) if unchanged, (_, err) on failure.
func (imp *Importer) importFile(ctx context.Context, path string) (bool, error) {
	relPath, err := filepath.Rel(imp.baseDir, path)
	if err != nil {
		return false, fmt.Errorf("rel path: %w", err)
	}

	hash, err := fileHash(path)
	if err != nil {
		return false, fmt.Errorf("hash: %w", err)
	}

	var existingHash string
	err = imp.db.QueryRowContext(ctx,
		"SELECT file_hash FROM books WHERE file_path = $1", relPath).Scan(&existingHash)
	if err == nil && existingHash == hash {
		return false, nil
	}

	log.Printf("import %s", relPath)

	book, err := parser.Parse(path)
	if err != nil {
		return false, fmt.Errorf("parse: %w", err)
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if strings.TrimSpace(book.Content) == "" {
		return false, fmt.Errorf("no text content extracted")
	}

	chunks := chunk(book.Content)
	if len(chunks) == 0 {
		return false, fmt.Errorf("no chunks")
	}

	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var bookID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO books (title, author, file_path, file_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (file_path) DO UPDATE
			SET title      = EXCLUDED.title,
			    author     = EXCLUDED.author,
			    file_hash  = EXCLUDED.file_hash,
			    created_at = NOW()
		RETURNING id`,
		book.Title, nullString(book.Author), relPath, hash,
	).Scan(&bookID)
	if err != nil {
		return false, fmt.Errorf("upsert book: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM chunks WHERE book_id = $1", bookID); err != nil {
		return false, fmt.Errorf("delete old chunks: %w", err)
	}

	for i, text := range chunks {
		vec, err := imp.embed.Embed(ctx, text)
		if err != nil {
			return false, fmt.Errorf("embed chunk %d: %w", i, err)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO chunks (book_id, content, embedding)
			VALUES ($1, $2, $3::vector)`,
			bookID, text, formatVector(vec),
		); err != nil {
			return false, fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	log.Printf("OK    %q by %q (%d chunks)", book.Title, book.Author, len(chunks))
	return true, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
