package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const model = "llama3.2"

var ValidGenres = []string{
	"Thriller", "Krimi", "Science Fiction", "Fantasy", "Horror",
	"Roman", "Spionage", "Klassiker", "Belletristik", "Gegenwartsliteratur",
	"Liebesroman", "Schmonzette", "Abenteuer", "Mystery", "Dystopie", "Action",
	"Detective", "Sachbuch", "Biografie", "Geschichte", "Humor",
}

var validGenreSet map[string]struct{}

func init() {
	validGenreSet = make(map[string]struct{}, len(ValidGenres))
	for _, g := range ValidGenres {
		validGenreSet[g] = struct{}{}
	}
}

const genreList = `Thriller, Krimi, Science Fiction, Fantasy, Horror, Roman, Spionage, Klassiker, Belletristik, Gegenwartsliteratur, Liebesroman, Schmonzette, Abenteuer, Mystery, Dystopie, Action, Detective, Sachbuch, Biografie, Geschichte, Humor`

const genreHint = `(Note: "Schmonzette" = trashy/cheesy romance or pulp fiction, e.g. Harlequin, Cora, Mills & Boon, erotic novels, Konsalik; "Liebesroman" = serious literary romance)`

const classifyPrompt = `Book: %q by %q
Task: Output exactly one genre from this list, nothing else:
` + genreList + `
` + genreHint + `
Genre:`

const normalizePrompt = `Existing genre label: %q
Map it to exactly one canonical genre from this list, nothing else:
` + genreList + `
` + genreHint + `
Genre:`

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{}}
}

func (c *Client) EnsureModel(ctx context.Context) error {
	body, _ := json.Marshal(map[string]any{"name": model, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pull %s: %w", model, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull %s: status %d", model, resp.StatusCode)
	}
	return nil
}

func (c *Client) Classify(ctx context.Context, title, author string) (string, error) {
	return c.generate(ctx, fmt.Sprintf(classifyPrompt, title, author))
}

func (c *Client) NormalizeGenre(ctx context.Context, genre string) (string, error) {
	return c.generate(ctx, fmt.Sprintf(normalizePrompt, genre))
}

func (c *Client) generate(ctx context.Context, prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"system": "You are a book genre classifier. You respond with exactly one genre from the given list. Never explain, never add punctuation.",
		"options": map[string]any{
			"num_predict": 10,
			"temperature": 0.1,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("generate: status %d", resp.StatusCode)
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	genre := strings.TrimSpace(result.Response)
	genre = strings.TrimRight(genre, ".")
	if _, ok := validGenreSet[genre]; !ok {
		return "", fmt.Errorf("unexpected genre %q", genre)
	}
	return genre, nil
}
