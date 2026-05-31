package parser

import (
	"io"
	"os"
	"strings"
)

// ParseMOBIMeta reads the PalmDB name field (first 32 bytes) as the title.
// Author is left empty so the importer falls back to the Calibre directory structure.
func ParseMOBIMeta(path string) (*Book, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var name [32]byte
	if _, err := io.ReadFull(f, name[:]); err != nil {
		return nil, err
	}
	title := CleanMeta(strings.TrimRight(string(name[:]), "\x00"))
	return &Book{Title: title}, nil
}

func ParseMOBI(path string) (*Book, error) {
	return ParseMOBIMeta(path)
}
