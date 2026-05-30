package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	dbpkg "github.com/pskiwi/ebook-search/internal/db"
)

type bookView struct {
	Title       string
	Author      string
	FilePath    string
	Format      string
	FormatClass string
}

var tmpl *template.Template

func Register(mux *http.ServeMux, db *sql.DB) {
	tmpl = template.Must(template.ParseGlob("web/templates/*.html"))

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/", indexHandler(db))
	mux.HandleFunc("/search", searchHandler(db))
}

func indexHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		books, err := dbpkg.ListBooks(r.Context(), db)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "index.html", toViews(books))
	}
}

func searchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		var (
			books []dbpkg.Book
			err   error
		)
		if q == "" {
			books, err = dbpkg.ListBooks(r.Context(), db)
		} else {
			books, err = dbpkg.SearchBooks(r.Context(), db, q)
		}
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "results", toViews(books))
	}
}

func toViews(books []dbpkg.Book) []bookView {
	views := make([]bookView, len(books))
	for i, b := range books {
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(b.FilePath), "."))
		views[i] = bookView{
			Title:       b.Title,
			Author:      b.Author,
			FilePath:    b.FilePath,
			Format:      ext,
			FormatClass: strings.ToLower(ext),
		}
	}
	return views
}
