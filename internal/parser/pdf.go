package parser

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ParsePDFMeta(path string) (*Book, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()
	return &Book{Title: pdfMeta(r, "Title"), Author: pdfMeta(r, "Author")}, nil
}

func ParsePDF(path string) (*Book, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
	}

	return &Book{
		Title:   pdfMeta(r, "Title"),
		Author:  pdfMeta(r, "Author"),
		Content: sb.String(),
	}, nil
}

func pdfMeta(r *pdf.Reader, key string) string {
	info := r.Trailer().Key("Info")
	if info.IsNull() {
		return ""
	}
	v := info.Key(key)
	if v.IsNull() {
		return ""
	}
	return v.String()
}
