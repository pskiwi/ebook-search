package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Book struct {
	Title   string
	Author  string
	Genre   string
	Content string
}

func Parse(path string) (*Book, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		return ParseEPUB(path)
	case ".pdf":
		return ParsePDF(path)
	default:
		return nil, fmt.Errorf("unsupported format: %s", filepath.Ext(path))
	}
}

// ParseMeta reads only title and author without extracting full text content.
// CleanMeta strips surrounding quotes and whitespace from PDF/EPUB metadata values.
func CleanMeta(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func ParseMeta(path string) (*Book, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		return ParseEPUBMeta(path)
	case ".pdf":
		return ParsePDFMeta(path)
	default:
		return nil, fmt.Errorf("unsupported format: %s", filepath.Ext(path))
	}
}
