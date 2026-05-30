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
