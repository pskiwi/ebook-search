package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pskiwi/ebook-search/internal/importer"
)

var (
	reLeadingNum = regexp.MustCompile(`^\d+\s*[-–]\s*`)
	reNumSep     = regexp.MustCompile(`\d+[_:]\s*(.+)$`)
)

var validExts = map[string]bool{
	".epub": true,
	".pdf":  true,
	".mobi": true,
	".azw3": true,
}

// Google Books API types — minimal, only what we need for metadata.
type gbVolumeInfo struct {
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
}

type gbVolume struct {
	VolumeInfo gbVolumeInfo `json:"volumeInfo"`
}

type gbResult struct {
	Items []gbVolume `json:"items"`
}

var gbClient = &http.Client{Timeout: 10 * time.Second}

const googleBooksAPI = "https://www.googleapis.com/books/v1/volumes"

// searchGoogleBooks tries multiple query strategies to find canonical metadata.
// Order: (1) cleaned title + author, (2) cleaned title only (author verified),
// (3) umlaut-normalized title + author, (4) umlaut-normalized title only (author verified).
func searchGoogleBooks(apiKey, title, author string) (gbVolumeInfo, bool, error) {
	cleaned := cleanSearchTitle(title)
	umlaut := normalizeUmlauts(cleaned)

	titles := []string{cleaned}
	if umlaut != cleaned {
		titles = append(titles, umlaut)
	}

	for _, t := range titles {
		// With author constraint — trust the result unconditionally.
		if author != "" && author != "Unknown" {
			vi, found, err := queryGoogleBooks(apiKey, t, author)
			if err != nil {
				return gbVolumeInfo{}, false, err
			}
			if found {
				return vi, true, nil
			}
		}
		// Without author constraint — verify the returned author matches our hint.
		vi, found, err := queryGoogleBooks(apiKey, t, "")
		if err != nil {
			return gbVolumeInfo{}, false, err
		}
		if found {
			returned := ""
			if len(vi.Authors) > 0 {
				returned = vi.Authors[0]
			}
			if author != "" && author != "Unknown" && !authorsMatch(author, returned) {
				continue // wrong book, try next title variant
			}
			return vi, true, nil
		}
	}
	return gbVolumeInfo{}, false, nil
}

// queryGoogleBooks performs a single API request and returns the first result.
func queryGoogleBooks(apiKey, title, author string) (gbVolumeInfo, bool, error) {
	time.Sleep(100 * time.Millisecond)
	q := fmt.Sprintf("intitle:%q", title)
	if author != "" && author != "Unknown" {
		q += fmt.Sprintf("+inauthor:%q", author)
	}
	u := googleBooksAPI + "?q=" + url.QueryEscape(q) + "&maxResults=1&fields=items(volumeInfo(title,authors))"
	if apiKey != "" {
		u += "&key=" + url.QueryEscape(apiKey)
	}
	resp, err := gbClient.Get(u)
	if err != nil {
		return gbVolumeInfo{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return gbVolumeInfo{}, false, fmt.Errorf("google books quota exceeded (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return gbVolumeInfo{}, false, fmt.Errorf("google books status %d", resp.StatusCode)
	}
	var result gbResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return gbVolumeInfo{}, false, err
	}
	if len(result.Items) > 0 && result.Items[0].VolumeInfo.Title != "" {
		return result.Items[0].VolumeInfo, true, nil
	}
	return gbVolumeInfo{}, false, nil
}

// authorsMatch checks whether the returned author shares a significant word with the guessed author.
func authorsMatch(guessed, returned string) bool {
	if returned == "" {
		return false
	}
	guessedLower := strings.ToLower(guessed)
	returnedLower := strings.ToLower(returned)
	for _, word := range strings.Fields(guessedLower) {
		if len(word) > 3 && strings.Contains(returnedLower, word) {
			return true
		}
	}
	return false
}

// extractMetaOllama uses a local Ollama model to extract author and title from a raw file path.
func extractMetaOllama(ctx context.Context, ollamaURL, path string) (author, title string, ok bool) {
	prompt := fmt.Sprintf(
		"Extract the book title and author from this file path. Respond with JSON only.\n\nFile path: %q\n\nRespond with exactly: {\"title\": \"...\", \"author\": \"...\"}",
		path,
	)
	body, _ := json.Marshal(map[string]any{
		"model":  "gemma4:27b",
		"prompt": prompt,
		"stream": false,
		"format": "json",
		"options": map[string]any{"temperature": 0.1},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", false
	}
	var meta struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	}
	if err := json.Unmarshal([]byte(result.Response), &meta); err != nil {
		return "", "", false
	}
	meta.Title = strings.TrimSpace(meta.Title)
	meta.Author = strings.TrimSpace(meta.Author)
	if meta.Title == "" {
		return "", "", false
	}
	return meta.Author, meta.Title, true
}

// normalizeUmlauts converts German ue/ae/oe to ü/ä/ö for better Google Books matching.
func normalizeUmlauts(s string) string {
	r := strings.NewReplacer(
		"ue", "ü", "ae", "ä", "oe", "ö",
		"Ue", "Ü", "Ae", "Ä", "Oe", "Ö",
	)
	return r.Replace(s)
}

// cleanSearchTitle strips series prefixes so Google Books finds the actual title.
// "Dirk Pitt 18 - Geheimcode Makaze" → "Geheimcode Makaze"
// "Die Oregon-Chroniken 1_ Der goldene Budd" → "Der goldene Budd"
// "18 - Some Title" → "Some Title"
func cleanSearchTitle(title string) string {
	// "Series Name 18 - Actual Title": last " - " with a digit before it
	if i := strings.LastIndex(title, " - "); i > 0 {
		if containsDigit(title[:i]) {
			return strings.TrimSpace(title[i+3:])
		}
	}
	// "Series 1_ Title" or "Series 1: Title"
	if m := reNumSep.FindStringSubmatch(title); m != nil {
		return strings.TrimSpace(m[1])
	}
	// Plain "18 - Title"
	return strings.TrimSpace(reLeadingNum.ReplaceAllString(title, ""))
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func main() {
	src := flag.String("src", "/Volumes/Daten/ebooks", "source directory (full library)")
	dst := flag.String("dst", "/Volumes/Daten/ebook-store", "target directory (new structure)")
	notFoundFile := flag.String("not-found", "not-found.tsv", "file to write books not found in Google Books")
	dryRun := flag.Bool("dry-run", false, "print what would happen without copying")
	flag.Parse()

	apiKey := os.Getenv("GOOGLE_BOOKS_API_KEY")
	if apiKey == "" {
		log.Println("WARN GOOGLE_BOOKS_API_KEY not set — requests may be rate-limited")
	}

	nf, err := os.Create(*notFoundFile)
	if err != nil {
		log.Fatalf("create %s: %v", *notFoundFile, err)
	}
	defer nf.Close()
	fmt.Fprintln(nf, "source_path\tguessed_title\tguessed_author")

	ollamaURL := os.Getenv("OLLAMA_URL")
	ctx := context.Background()

	var copied, skipped, conflicts, notFound, ollamaExtracted int

	err = filepath.WalkDir(*src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), "._") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !validExts[ext] {
			return nil
		}

		// Extract metadata hints from path.
		guessedAuthor := normalizeAuthor(filepath.Base(filepath.Dir(filepath.Dir(path))))
		guessedTitle := importer.TitleFromPath(path)
		if guessedTitle == "" {
			guessedTitle = importer.TitleFromFilename(path)
		}
		// If author dir == title dir, Calibre had no author set.
		if guessedAuthor == guessedTitle {
			guessedAuthor = ""
		}

		// Query Google Books.
		vi, found, err := searchGoogleBooks(apiKey, guessedTitle, guessedAuthor)
		if err != nil {
			log.Printf("WARN google books query for %q: %v", guessedTitle, err)
		}

		var author, title string
		if found {
			title = sanitize(vi.Title)
			if len(vi.Authors) > 0 {
				author = sanitize(vi.Authors[0])
			}
		}
		if !found || title == "" {
			// Fallback: use Ollama to extract author+title from the raw path.
			if ollamaURL != "" {
				ollamaAuthor, ollamaTitle, ollamaOK := extractMetaOllama(ctx, ollamaURL, relPath(*src, path))
				if ollamaOK {
					author = sanitize(ollamaAuthor)
					title = sanitize(ollamaTitle)
					fmt.Printf("[OLLAMA]    %s → %s/%s\n", relPath(*src, path), author, title)
					ollamaExtracted++
				}
			}
			if title == "" {
				fmt.Fprintf(nf, "%s\t%s\t%s\n", path, guessedTitle, guessedAuthor)
				fmt.Printf("[NOT FOUND] %s\n", relPath(*src, path))
				notFound++
				return nil
			}
		}
		if author == "" {
			author = "Unknown"
		}

		destPath := filepath.Join(*dst, author, title, "book"+ext)
		rel := fmt.Sprintf("%s/%s/book%s", author, title, ext)

		if *dryRun {
			fmt.Printf("[DRY]       %s\n", rel)
			copied++
			return nil
		}

		if _, statErr := os.Stat(destPath); statErr == nil {
			srcHash, err := hashFile(path)
			if err != nil {
				log.Printf("WARN hash src %s: %v", path, err)
				return nil
			}
			dstHash, err := hashFile(destPath)
			if err != nil {
				log.Printf("WARN hash dst %s: %v", destPath, err)
				return nil
			}
			if srcHash == dstHash {
				fmt.Printf("[SKIP]      %s\n", rel)
				skipped++
			} else {
				fmt.Printf("[CONFLICT]  %s\n", rel)
				conflicts++
			}
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
		}
		if err := copyFile(path, destPath); err != nil {
			return fmt.Errorf("copy %s → %s: %w", path, destPath, err)
		}
		fmt.Printf("[COPY]      %s\n", rel)
		copied++
		return nil
	})
	if err != nil {
		log.Fatalf("walk: %v", err)
	}

	fmt.Printf("\nFertig: %d kopiert, %d übersprungen, %d Konflikte, %d Ollama, %d nicht gefunden (→ %s)\n",
		copied, skipped, conflicts, ollamaExtracted, notFound, *notFoundFile)
}

func relPath(base, full string) string {
	r, err := filepath.Rel(base, full)
	if err != nil {
		return full
	}
	return r
}

func normalizeAuthor(s string) string {
	if i := strings.Index(s, ", "); i > 0 {
		return strings.TrimSpace(s[i+2:]) + " " + s[:i]
	}
	return s
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', '?', '*', '"', '<', '>', '|':
			// skip
		case ':':
			b.WriteString(" -")
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
