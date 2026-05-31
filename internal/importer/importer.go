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
	"regexp"
	"strings"
	"unicode"

	"github.com/pskiwi/ebook-search/internal/parser"
)

type Classifier interface {
	Classify(ctx context.Context, title, author string) (string, error)
}

type Importer struct {
	db         *sql.DB
	baseDir    string
	classifier Classifier
}

func New(db *sql.DB, baseDir string) *Importer {
	return &Importer{db: db, baseDir: baseDir}
}

func (imp *Importer) WithClassifier(c Classifier) *Importer {
	imp.classifier = c
	return imp
}

func (imp *Importer) Run(ctx context.Context, dir string) error {
	fmt.Fprintf(os.Stderr, "Zähle Dateien in %s…\n", dir)
	total, err := countFiles(dir)
	if err != nil {
		return fmt.Errorf("count files: %w", err)
	}
	fmt.Fprintf(os.Stderr, "%d Bücher gefunden\n", total)

	var processed, imported, skipped, failed int
	printProgress := func() {
		fmt.Fprintf(os.Stderr, "\r[%d/%d] importiert: %d  unverändert: %d  fehler: %d   ",
			processed, total, imported, skipped, failed)
	}

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "._") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".epub" && ext != ".pdf" && ext != ".mobi" {
			return nil
		}
		processed++
		label, changed, importErr := imp.importFile(ctx, path)
		switch {
		case importErr != nil:
			fmt.Fprintf(os.Stderr, "\nFAIL  %s: %v\n", filepath.Base(path), importErr)
			if dbErr := imp.recordError(ctx, path, importErr); dbErr != nil {
				fmt.Fprintf(os.Stderr, "\nWARN  could not record error: %v\n", dbErr)
			}
			failed++
		case !changed:
			skipped++
		default:
			log.Printf("\nOK    %s", label)
			imported++
		}
		printProgress()
		return nil
	})
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Fertig: %d importiert, %d unverändert, %d fehler\n", imported, skipped, failed)
	return err
}

func countFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	type result struct {
		n   int
		err error
	}

	// collect top-level subdirs to walk in parallel
	var subdirs []string
	var topFiles int
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, filepath.Join(dir, e.Name()))
		} else {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".epub" || ext == ".pdf" {
				topFiles++
			}
		}
	}

	ch := make(chan result, len(subdirs))
	for _, sub := range subdirs {
		go func(d string) {
			n, err := countFilesSeq(d)
			ch <- result{n, err}
		}(sub)
	}

	total := topFiles
	for range subdirs {
		r := <-ch
		if r.err != nil {
			return 0, r.err
		}
		total += r.n
	}
	return total, nil
}

func countFilesSeq(dir string) (int, error) {
	var count int
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == ".epub" || ext == ".pdf" {
				count++
			}
		}
		return nil
	})
	return count, err
}

func (imp *Importer) importFile(ctx context.Context, path string) (string, bool, error) {
	relPath, err := filepath.Rel(imp.baseDir, path)
	if err != nil {
		return "", false, fmt.Errorf("rel path: %w", err)
	}

	hash, err := fileHash(path)
	if err != nil {
		return "", false, fmt.Errorf("hash: %w", err)
	}

	var existingHash string
	err = imp.db.QueryRowContext(ctx,
		"SELECT file_hash FROM books WHERE file_path = $1", relPath).Scan(&existingHash)
	if err == nil && existingHash == hash {
		return "", false, nil
	}

	book, err := parser.ParseMeta(path)
	if err != nil {
		return "", false, fmt.Errorf("parse meta: %w", err)
	}

	book.Title = parser.CleanMeta(book.Title)
	book.Author = parser.CleanMeta(book.Author)

	if book.Title == "" || looksLikeGarbage(book.Title) || looksLikeSeriesNumber(book.Title) {
		if better := TitleFromPath(path); better != "" {
			book.Title = better
		}
	}
	if book.Author == "" {
		book.Author = authorFromPath(path)
	}

	if book.Genre == "" && imp.classifier != nil {
		genre, err := imp.classifier.Classify(ctx, book.Title, book.Author)
		if err != nil {
			log.Printf("WARN  classify %q: %v", book.Title, err)
		} else {
			book.Genre = genre
		}
	}

	_, err = imp.db.ExecContext(ctx, `
		INSERT INTO books (title, author, file_path, file_hash, genre)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (file_path) DO UPDATE
			SET title      = EXCLUDED.title,
			    author     = EXCLUDED.author,
			    file_hash  = EXCLUDED.file_hash,
			    genre      = EXCLUDED.genre,
			    created_at = NOW()`,
		book.Title, nullString(book.Author), relPath, hash, nullString(book.Genre),
	)
	if err != nil {
		return "", false, fmt.Errorf("upsert book: %w", err)
	}

	return fmt.Sprintf("%q — %q", book.Title, book.Author), true, nil
}


// TitleFromPath extracts the title from Calibre's "Title (ID)" parent directory,
// falling back to the cleaned filename, then the author directory.
func TitleFromPath(path string) string {
	parent := filepath.Base(filepath.Dir(path))

	// Calibre convention: parent dir is "Title (ID)" — reliable title source
	if i := strings.LastIndex(parent, " ("); i > 0 {
		candidate := strings.TrimSpace(parent[:i])
		candidate = strings.ReplaceAll(candidate, "_ ", ": ")
		candidate = strings.ReplaceAll(candidate, "_", " ")
		if !looksLikeGarbage(candidate) {
			return candidate
		}
	}

	// Non-Calibre: try the filename first (more specific than a collection dir)
	if name := titleFromFilename(path); !looksLikeGarbage(name) && len(name) > 0 {
		return name
	}

	// Fall back to parent directory name
	parent = strings.ReplaceAll(parent, "_ ", ": ")
	parent = strings.ReplaceAll(parent, "_", " ")
	if !looksLikeGarbage(parent) {
		return parent
	}

	return authorFromPath(path)
}

// TitleFromFilename strips the extension and "Author - " prefix from the filename.
func TitleFromFilename(path string) string {
	return titleFromFilename(path)
}

func titleFromFilename(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ReplaceAll(name, "_", " ")
	if i := strings.Index(name, " - "); i > 0 {
		candidate := strings.TrimSpace(name[i+3:])
		if !looksLikeGarbage(candidate) {
			return candidate
		}
	}
	return strings.TrimSpace(name)
}

// authorFromPath uses the grandparent directory as author (Calibre: Author/Title/file).
func authorFromPath(path string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(path)))
}

var placeholders = map[string]bool{
	"job": true, "document": true, "untitled": true, "doc": true,
	"unknown": true, "noname": true, "test": true, "none": true,
}

// looksLikeSeriesNumber matches titles like "Dino-Land 12", "Belgariad 01", "Layout 1"
// — a word (possibly hyphenated) followed only by digits, with no subtitle.
var reSeriesNumber = regexp.MustCompile(`(?i)^[\p{L}\d][\p{L}\d\-]* \d+$`)

func looksLikeSeriesNumber(s string) bool {
	return reSeriesNumber.MatchString(strings.TrimSpace(s))
}

func looksLikeGarbage(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return true
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, ".rtf") || strings.Contains(lower, ".doc") ||
		strings.Contains(lower, ".epub") || strings.Contains(lower, ".pdf") ||
		strings.HasPrefix(lower, "microsoft word") {
		return true
	}
	if placeholders[lower] {
		return true
	}
	// pure digits (IDs, ISBNs)
	allDigits := true
	for _, r := range s {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	// PDF anchor/bookmark IDs (e.g. "#000LaRosaCip") → not a real title
	if strings.HasPrefix(s, "#") {
		return true
	}
	// control characters → binary garbage from broken PDF metadata
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	// no letters at all (e.g. "-", "---", "42") → not a real title
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return !hasLetter
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

func (imp *Importer) recordError(ctx context.Context, path string, importErr error) error {
	relPath, err := filepath.Rel(imp.baseDir, path)
	if err != nil {
		relPath = path
	}
	_, err = imp.db.ExecContext(ctx, `
		INSERT INTO import_errors (file_path, error)
		VALUES ($1, $2)
		ON CONFLICT (file_path) DO UPDATE SET error = EXCLUDED.error, created_at = NOW()`,
		relPath, importErr.Error())
	return err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
