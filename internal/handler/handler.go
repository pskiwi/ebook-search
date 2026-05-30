package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	dbpkg "github.com/pskiwi/ebook-search/internal/db"
)

type formatEntry struct {
	ID          int64
	Format      string
	FormatClass string
}

type bookView struct {
	ID          int64
	Title       string
	Author      string
	Genre       string
	FilePath    string
	Format      string
	FormatClass string
	Description string
	Formats     []formatEntry
	InList      bool
	ListCount   int
}

type indexView struct {
	resultsView
	Genres        []string
	TotalCount    int
	ListCount     int
}

type listView struct {
	Books []bookView
}

type resultsView struct {
	Books      []bookView
	HasMore    bool
	NextOffset int
	Query      string
	Genre      string
}

var tmpl *template.Template

func Register(mux *http.ServeMux, db *sql.DB) {
	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"urlquery": url.QueryEscape,
	}).ParseGlob("web/templates/*.html"))

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/", indexHandler(db))
	mux.HandleFunc("/search", searchHandler(db))
	mux.HandleFunc("/book/", bookHandler(db))
	mux.HandleFunc("/list", listHandler(db))
	mux.HandleFunc("/list/toggle/", listToggleHandler(db))
}

func indexHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		books, err := dbpkg.ListBooks(r.Context(), db, "", 0)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		total, err := dbpkg.CountBooks(r.Context(), db)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		listCount, err := dbpkg.CountReadingList(r.Context(), db)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "index.html", indexView{
			resultsView: buildView(books, 0, "", ""),
			Genres:      genres,
			TotalCount:  total,
			ListCount:   listCount,
		})
	}
}

var genres = []string{
	"Abenteuer", "Action", "Belletristik", "Biografie", "Detective",
	"Dystopie", "Fantasy", "Gegenwartsliteratur", "Geschichte", "Horror",
	"Humor", "Klassiker", "Krimi", "Liebesroman", "Mystery",
	"Roman", "Sachbuch", "Schmonzette", "Science Fiction", "Spionage", "Thriller",
}

func bookHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/book/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		book, err := dbpkg.GetBook(r.Context(), db, id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		inList, _ := dbpkg.IsInReadingList(r.Context(), db, id)
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(book.FilePath), "."))
		view := bookView{
			ID:          book.ID,
			Title:       book.Title,
			Author:      book.Author,
			Genre:       book.Genre,
			FilePath:    book.FilePath,
			Format:      ext,
			FormatClass: strings.ToLower(ext),
			Description: book.Description,
			InList:      inList,
		}
		tmpl.ExecuteTemplate(w, "book-detail", view)
	}
}

func listToggleHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/list/toggle/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		inList, err := dbpkg.ToggleReadingList(r.Context(), db, id)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		count, _ := dbpkg.CountReadingList(r.Context(), db)
		view := bookView{ID: id, InList: inList, ListCount: count}
		tmpl.ExecuteTemplate(w, "list-toggle-btn", view)
		tmpl.ExecuteTemplate(w, "list-count-oob", view)
	}
}

func listHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		books, err := dbpkg.GetReadingList(r.Context(), db)
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "list.html", listView{Books: toViews(books)})
	}
}

func searchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		genre := strings.TrimSpace(r.URL.Query().Get("genre"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		var (
			books []dbpkg.Book
			err   error
		)
		if q == "" {
			books, err = dbpkg.ListBooks(r.Context(), db, genre, offset)
		} else {
			books, err = dbpkg.SearchBooks(r.Context(), db, q, genre, offset)
		}
		if err != nil {
			http.Error(w, "Datenbankfehler", http.StatusInternalServerError)
			return
		}

		view := buildView(books, offset, q, genre)
		if offset > 0 {
			tmpl.ExecuteTemplate(w, "more-results", view)
		} else {
			tmpl.ExecuteTemplate(w, "results", view)
		}
	}
}

func buildView(books []dbpkg.Book, offset int, query, genre string) resultsView {
	hasMore := len(books) > 100
	if hasMore {
		books = books[:100]
	}
	return resultsView{
		Books:      toViews(books),
		HasMore:    hasMore,
		NextOffset: offset + 100,
		Query:      query,
		Genre:      genre,
	}
}

func toViews(books []dbpkg.Book) []bookView {
	type key struct{ title, author string }
	seen := make(map[key]int)
	var views []bookView

	for _, b := range books {
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(b.FilePath), "."))
		k := key{b.Title, b.Author}
		fe := formatEntry{ID: b.ID, Format: ext, FormatClass: strings.ToLower(ext)}

		if idx, ok := seen[k]; ok {
			views[idx].Formats = append(views[idx].Formats, fe)
		} else {
			seen[k] = len(views)
			views = append(views, bookView{
				ID:          b.ID,
				Title:       b.Title,
				Author:      b.Author,
				Genre:       b.Genre,
				FilePath:    b.FilePath,
				Format:      ext,
				FormatClass: strings.ToLower(ext),
				Formats:     []formatEntry{fe},
			})
		}
	}
	return views
}
