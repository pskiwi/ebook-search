package parser

import "io"

type Book struct {
	Title   string
	Author  string
	Content string
}

// ParseEPUB extracts text content from an EPUB file.
func ParseEPUB(r io.Reader) (*Book, error) {
	// TODO: implement
	return nil, nil
}

// ParsePDF extracts text content from a PDF file.
func ParsePDF(r io.Reader) (*Book, error) {
	// TODO: implement
	return nil, nil
}
