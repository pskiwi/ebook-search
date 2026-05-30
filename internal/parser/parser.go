package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Book struct {
	Title   string
	Author  string
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
