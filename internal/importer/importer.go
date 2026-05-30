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
	"strings"

	"github.com/pskiwi/ebook-search/internal/parser"
)

type Importer struct {
	db      *sql.DB
	baseDir string
}

func New(db *sql.DB, baseDir string) *Importer {
	return &Importer{db: db, baseDir: baseDir}
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

	book, err := parser.ParseMeta(path)
	if err != nil {
		return false, fmt.Errorf("parse meta: %w", err)
	}

	book.Title = parser.CleanMeta(book.Title)
	book.Author = parser.CleanMeta(book.Author)

	if book.Title == "" || looksLikeFilename(book.Title) {
		book.Title = titleFromPath(path)
	}
	if book.Author == "" {
		book.Author = authorFromPath(path)
	}

	_, err = imp.db.ExecContext(ctx, `
		INSERT INTO books (title, author, file_path, file_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (file_path) DO UPDATE
			SET title      = EXCLUDED.title,
			    author     = EXCLUDED.author,
			    file_hash  = EXCLUDED.file_hash,
			    created_at = NOW()`,
		book.Title, nullString(book.Author), relPath, hash,
	)
	if err != nil {
		return false, fmt.Errorf("upsert book: %w", err)
	}

	log.Printf("OK    %q — %q", book.Title, book.Author)
	return true, nil
}

// titleFromPath extracts the title from Calibre's "Title (ID)" parent directory.
// Falls back to grandparent if parent also looks like a filename.
func titleFromPath(path string) string {
	parent := filepath.Base(filepath.Dir(path))
	if i := strings.LastIndex(parent, " ("); i > 0 {
		parent = strings.TrimSpace(parent[:i])
	}
	if looksLikeFilename(parent) {
		return authorFromPath(path)
	}
	return parent
}

// authorFromPath uses the grandparent directory as author (Calibre: Author/Title/file).
func authorFromPath(path string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(path)))
}

func looksLikeFilename(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, ".rtf") ||
		strings.Contains(lower, ".doc") ||
		strings.HasPrefix(lower, "microsoft word")
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
