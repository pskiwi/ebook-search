package parser

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ParsePDFMeta(path string) (book *Book, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pdf panic: %v", r)
		}
	}()
	f, r, openErr := pdf.Open(path)
	if openErr != nil {
		return nil, fmt.Errorf("open pdf: %w", openErr)
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
	return decodePDFString(v.String())
}

// decodePDFString handles UTF-16BE strings (marked by a \xfe\xff BOM).
// All other strings are returned as-is.
func decodePDFString(s string) string {
	b := []byte(s)
	if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
		runes := make([]rune, 0, len(b)/2)
		for i := 2; i+1 < len(b); i += 2 {
			runes = append(runes, rune(b[i])<<8|rune(b[i+1]))
		}
		return string(runes)
	}
	return s
}
