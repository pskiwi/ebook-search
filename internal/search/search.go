package search

import "database/sql"

type Result struct {
	ID      int
	Title   string
	Author  string
	Snippet string
	Score   float64
}

type Searcher struct {
	db *sql.DB
}

func New(db *sql.DB) *Searcher {
	return &Searcher{db: db}
}

// Hybrid combines full-text and vector search results.
func (s *Searcher) Hybrid(query string, embedding []float32) ([]Result, error) {
	// TODO: implement
	return nil, nil
}
