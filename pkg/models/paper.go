package models

import (
	"fmt"
	"strings"
	"unicode"
)

type Paper struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Year        string   `json:"year"`
	DOI         string   `json:"doi"`
	URL         string   `json:"url"`
	Venue       string   `json:"venue"`
	Journal     string   `json:"journal,omitempty"`
	Volume      string   `json:"volume,omitempty"`
	Issue       string   `json:"issue,omitempty"`
	Pages       string   `json:"pages,omitempty"`
	Publisher   string   `json:"publisher,omitempty"`
	Citations   int      `json:"citations"`
	Abstract    string   `json:"abstract"`
	Source      string   `json:"source"`
	PDFURL      string   `json:"pdf_url"`
	BibTeXKey   string   `json:"bibtex_key"`
	Type        string   `json:"type"`
	ArXivID     string   `json:"arxiv_id"`
}

type SearchResult struct {
	Query        string   `json:"query"`
	TotalResults int      `json:"total_results"`
	Results      []Paper  `json:"results"`
	SourcesUsed  []string `json:"sources_used"`
}

type ReadResult struct {
	Identifier   string  `json:"identifier"`
	IDType       string  `json:"id_type"`
	Metadata     *Paper  `json:"metadata"`
	Abstract     string  `json:"abstract"`
	FullText     string  `json:"full_text"`
	Conclusions  string  `json:"conclusions"`
	SourceOfText string  `json:"source_of_text"`
	Error        *string `json:"error"`
}

func MakeBibTeXKey(authors []string, year string, title string) string {
	last := "unknown"
	if len(authors) > 0 {
		parts := strings.Fields(authors[0])
		if len(parts) > 0 {
			last = parts[len(parts)-1]
		}
	}
	y := year
	if y == "" {
		y = "????"
	}
	firstWord := "x"
	if t := strings.TrimSpace(title); t != "" {
		words := strings.Fields(t)
		if len(words) > 0 {
			clean := strings.TrimFunc(words[0], func(r rune) bool {
				return unicode.IsPunct(r) || unicode.IsSymbol(r)
			})
			if clean != "" {
				firstWord = strings.ToLower(clean)
			}
		}
	}
	return fmt.Sprintf("%s%s%s", last, y, firstWord)
}
