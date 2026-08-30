package sources

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hossam1522/VerifiSci/pkg/cache"
	"github.com/hossam1522/VerifiSci/pkg/client"
	"github.com/hossam1522/VerifiSci/pkg/models"
)

const arXivBase = "http://export.arxiv.org/api/query"

type arXivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arXivEntry `xml:"entry"`
}

type arXivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []arXivAuthor `xml:"author"`
}

type arXivAuthor struct {
	Name string `xml:"name"`
}

func parseArXivEntry(entry arXivEntry) models.Paper {
	title := strings.Join(strings.Fields(entry.Title), " ")
	abstract := strings.Join(strings.Fields(entry.Summary), " ")

	var authors []string
	for _, a := range entry.Authors {
		if n := strings.TrimSpace(a.Name); n != "" {
			authors = append(authors, n)
		}
	}

	yearStr := ""
	if len(entry.Published) >= 4 {
		yearStr = entry.Published[:4]
	}

	rawID := strings.TrimSpace(entry.ID)
	arxivID := strings.TrimPrefix(rawID, "http://arxiv.org/abs/")
	arxivID = strings.TrimPrefix(arxivID, "https://arxiv.org/abs/")
	pdfURL := fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", arxivID)

	return models.Paper{
		Title:     title,
		Authors:   authors,
		Year:      yearStr,
		DOI:       "",
		URL:       rawID,
		Venue:     "arXiv",
		Citations: 0,
		Abstract:  abstract,
		Source:    "arxiv",
		PDFURL:    pdfURL,
		BibTeXKey: models.MakeBibTeXKey(authors, yearStr, title),
		Type:      "article",
		ArXivID:   arxivID,
	}
}

func normalizeString(s string) string {
	reg := regexp.MustCompile(`[^\w\s]`)
	cleaned := reg.ReplaceAllString(s, "")
	return strings.ToLower(strings.Join(strings.Fields(cleaned), " "))
}

func EnrichPaperMetadata(paper *models.Paper, semanticKey string) {
	if paper == nil || paper.Title == "" {
		return
	}
	if paper.Citations > 0 && paper.DOI != "" {
		return
	}

	// 1. Try Semantic Scholar if key available
	if semanticKey != "" && paper.ArXivID != "" {
		if sp, err := GetSemanticPaper(paper.ArXivID, "ARXIV", semanticKey); err == nil && sp != nil {
			if paper.Citations == 0 && sp.Citations > 0 {
				paper.Citations = sp.Citations
			}
			if paper.DOI == "" && sp.DOI != "" {
				paper.DOI = sp.DOI
			}
			if (paper.Venue == "" || paper.Venue == "arXiv") && sp.Venue != "" {
				paper.Venue = sp.Venue
			}
			return
		}
	}

	// 2. Try OpenAlex title search
	normTitle := normalizeString(paper.Title)
	if searchResults, err := SearchOpenAlex(paper.Title, 3, "", "", "relevance"); err == nil {
		for _, item := range searchResults {
			itemNorm := normalizeString(item.Title)
			if itemNorm == normTitle || (len(normTitle) > 10 && strings.Contains(itemNorm, normTitle)) {
				if paper.Citations == 0 && item.Citations > 0 {
					paper.Citations = item.Citations
				}
				if paper.DOI == "" && item.DOI != "" {
					paper.DOI = item.DOI
				}
				return
			}
		}
		if len(searchResults) > 0 {
			if paper.Citations == 0 && searchResults[0].Citations > 0 {
				paper.Citations = searchResults[0].Citations
			}
			if paper.DOI == "" && searchResults[0].DOI != "" {
				paper.DOI = searchResults[0].DOI
			}
		}
	}
}

func SearchArXiv(query string, limit int, semanticKey string) ([]models.Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	cacheK := cache.Key("arxiv_search", query, strconv.Itoa(limit))
	if cached, ok := cache.Get[[]models.Paper](cacheK, time.Hour); ok {
		return *cached, nil
	}

	params := url.Values{}
	params.Set("search_query", fmt.Sprintf("all:%s", query))
	params.Set("start", "0")
	params.Set("max_results", strconv.Itoa(limit))
	params.Set("sortBy", "relevance")

	reqURL := fmt.Sprintf("%s?%s", arXivBase, params.Encode())
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var feed arXivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	var results []models.Paper
	for _, entry := range feed.Entries {
		p := parseArXivEntry(entry)
		EnrichPaperMetadata(&p, semanticKey)
		results = append(results, p)
	}

	_ = cache.Set(cacheK, results)
	return results, nil
}

func GetArXivPaper(arxivID string, semanticKey string) (*models.Paper, error) {
	cacheK := cache.Key("arxiv_paper", arxivID)
	if cached, ok := cache.Get[models.Paper](cacheK, time.Hour); ok {
		return cached, nil
	}

	reqURL := fmt.Sprintf("%s?id_list=%s&max_results=1", arXivBase, url.QueryEscape(arxivID))
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var feed arXivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("paper not found on arXiv")
	}

	paper := parseArXivEntry(feed.Entries[0])
	EnrichPaperMetadata(&paper, semanticKey)

	_ = cache.Set(cacheK, paper)
	return &paper, nil
}
