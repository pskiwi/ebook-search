package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	reNumPrefix  = regexp.MustCompile(`^\d+\s*[-–]\s*`)
	reParenSuffix = regexp.MustCompile(`\s*\([^)]+\)\s*$`)
)

const googleBooksAPI = "https://www.googleapis.com/books/v1/volumes"

var gbClient = &http.Client{Timeout: 8 * time.Second}

type gbVolumeInfo struct {
	Publisher     string   `json:"publisher"`
	PublishedDate string   `json:"publishedDate"`
	Description   string   `json:"description"`
	PageCount     int      `json:"pageCount"`
	Categories    []string `json:"categories"`
	ImageLinks    struct {
		Thumbnail string `json:"thumbnail"`
	} `json:"imageLinks"`
	IndustryIdentifiers []struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	} `json:"industryIdentifiers"`
	InfoLink string `json:"infoLink"`
}

type gbVolume struct {
	VolumeInfo gbVolumeInfo `json:"volumeInfo"`
}

type gbResult struct {
	Items []gbVolume `json:"items"`
}

type bookInfoView struct {
	Thumbnail     string
	Publisher     string
	PublishedDate string
	PageCount     int
	Categories    string
	ISBN          string
	Description   string
	InfoLink      string
	NotFound      bool
}

func cleanTitle(title string) string {
	t := reNumPrefix.ReplaceAllString(title, "")
	t = reParenSuffix.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

func fetchGoogleBooksInfo(apiKey, title, author string) (bookInfoView, error) {
	q := fmt.Sprintf("intitle:%q", cleanTitle(title))
	if author != "" {
		q += fmt.Sprintf("+inauthor:%q", author)
	}
	reqURL := googleBooksAPI + "?q=" + url.QueryEscape(q) + "&maxResults=1"
	if apiKey != "" {
		reqURL += "&key=" + url.QueryEscape(apiKey)
	}

	resp, err := gbClient.Get(reqURL)
	if err != nil {
		return bookInfoView{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return bookInfoView{}, fmt.Errorf("google books status %d", resp.StatusCode)
	}

	var result gbResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return bookInfoView{}, err
	}
	if len(result.Items) == 0 {
		return bookInfoView{NotFound: true}, nil
	}

	vi := result.Items[0].VolumeInfo

	thumbnail := vi.ImageLinks.Thumbnail
	if strings.HasPrefix(thumbnail, "http://") {
		thumbnail = "https://" + thumbnail[7:]
	}

	var isbn string
	for _, id := range vi.IndustryIdentifiers {
		if id.Type == "ISBN_13" {
			isbn = id.Identifier
			break
		}
		if id.Type == "ISBN_10" && isbn == "" {
			isbn = id.Identifier
		}
	}

	return bookInfoView{
		Thumbnail:     thumbnail,
		Publisher:     vi.Publisher,
		PublishedDate: vi.PublishedDate,
		PageCount:     vi.PageCount,
		Categories:    strings.Join(vi.Categories, ", "),
		ISBN:          isbn,
		Description:   vi.Description,
		InfoLink:      vi.InfoLink,
	}, nil
}
