package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pskiwi/ebook-search/internal/importer"
)

var validExts = map[string]bool{
	".epub": true,
	".pdf":  true,
	".mobi": true,
}

func main() {
	src := flag.String("src", "/Volumes/Daten/ebooks/ebooks", "Calibre source directory")
	dst := flag.String("dst", "/Volumes/Daten/ebook-store", "target directory (new structure)")
	dryRun := flag.Bool("dry-run", false, "print what would happen without copying")
	flag.Parse()

	var copied, skipped, conflicts int

	err := filepath.WalkDir(*src, func(path string, d os.DirEntry, err error) error {
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

		author := sanitize(normalizeAuthor(filepath.Base(filepath.Dir(filepath.Dir(path)))))
		title := sanitize(importer.TitleFromPath(path))
		if title == "" {
			title = sanitize(importer.TitleFromFilename(path))
		}
		if author == "" {
			author = "Unknown"
		}
		if title == "" {
			title = strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		}

		destPath := filepath.Join(*dst, author, title, "book"+ext)
		rel := fmt.Sprintf("%s/%s/book%s", author, title, ext)

		if *dryRun {
			fmt.Printf("[DRY]      %s\n", rel)
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
				fmt.Printf("[SKIP]     %s\n", rel)
				skipped++
			} else {
				fmt.Printf("[CONFLICT] %s\n", rel)
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
		fmt.Printf("[COPY]     %s\n", rel)
		copied++
		return nil
	})
	if err != nil {
		log.Fatalf("walk: %v", err)
	}

	if *dryRun {
		fmt.Printf("\nDry-run: %d würden kopiert werden\n", copied)
	} else {
		fmt.Printf("\nFertig: %d kopiert, %d übersprungen, %d Konflikte\n", copied, skipped, conflicts)
	}
}

// normalizeAuthor converts "Lastname, Firstname" to "Firstname Lastname".
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
