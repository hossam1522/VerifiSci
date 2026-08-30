package sources

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/hossam1522/VerifiSci/pkg/cache"
	"github.com/hossam1522/VerifiSci/pkg/client"
	"github.com/hossam1522/VerifiSci/pkg/models"
)

const semanticBase = "https://api.semanticscholar.org/graph/v1"

type semanticSearchResponse struct {
	Data []semanticPaper `json:"data"`
}

type semanticPaper struct {
	PaperID       string              `json:"paperId"`
	Title         string              `json:"title"`
	Year          int                 `json:"year"`
	Abstract      string              `json:"abstract"`
	URL           string              `json:"url"`
	Venue         string              `json:"venue"`
	CitationCount int                 `json:"citationCount"`
	Authors       []semanticAuthor    `json:"authors"`
	ExternalIDs   map[string]string   `json:"externalIds"`
	OpenAccessPDF *semanticPDF        `json:"openAccessPdf"`
	Journal       *semanticJournal    `json:"journal"`
	TLDR          *semanticTLDR       `json:"tldr"`
}

type semanticAuthor struct {
	Name string `json:"name"`
}

type semanticPDF struct {
	URL string `json:"url"`
}

type semanticJournal struct {
	Name   string `json:"name"`
	Volume string `json:"volume"`
	Pages  string `json:"pages"`
}

type semanticTLDR struct {
	Text string `json:"text"`
}

func parseSemanticPaper(p semanticPaper) models.Paper {
	var authors []string
	for _, a := range p.Authors {
		if a.Name != "" {
			authors = append(authors, a.Name)
		}
	}

	doi := ""
	arxivID := ""
	if p.ExternalIDs != nil {
		doi = p.ExternalIDs["DOI"]
		arxivID = p.ExternalIDs["ArXiv"]
	}

	paperURL := p.URL
	if doi != "" {
		paperURL = fmt.Sprintf("https://doi.org/%s", doi)
	}

	pdfURL := ""
	if p.OpenAccessPDF != nil {
		pdfURL = p.OpenAccessPDF.URL
	}

	journalName := ""
	volume := ""
	pages := ""
	if p.Journal != nil {
		journalName = p.Journal.Name
		volume = p.Journal.Volume
		pages = p.Journal.Pages
	}

	yearStr := ""
	if p.Year > 0 {
		yearStr = strconv.Itoa(p.Year)
	}

	abstract := p.Abstract
	if abstract == "" && p.TLDR != nil {
		abstract = p.TLDR.Text
	}

	return models.Paper{
		Title:       p.Title,
		Authors:     authors,
		Year:        yearStr,
		DOI:         doi,
		URL:         paperURL,
		Venue:       p.Venue,
		Journal:     journalName,
		Volume:      volume,
		Pages:       pages,
		Citations:   p.CitationCount,
		Abstract:    abstract,
		Source:      "semantic_scholar",
		PDFURL:      pdfURL,
		BibTeXKey:   models.MakeBibTeXKey(authors, yearStr, p.Title),
		Type:        "article",
		ArXivID:     arxivID,
	}
}

func getSemanticHeaders(apiKey string) map[string]string {
	headers := map[string]string{
		"User-Agent":      "PostmanRuntime/7.56.1",
		"Accept":          "*/*",
		"Cache-Control":   "no-cache",
		"Accept-Encoding": "gzip, deflate, br",
	}
	if apiKey != "" {
		headers["x-api-key"] = apiKey
	}
	return headers
}

func SearchSemanticScholar(query string, limit int, yearFrom string, yearTo string, sortOrder string, apiKey string) ([]models.Paper, error) {
	if apiKey == "" {
		return nil, nil // Skip silently when no API key provided
	}
	if limit <= 0 {
		limit = 10
	}

	cacheK := cache.Key("semantic_search", query, strconv.Itoa(limit), yearFrom, yearTo, sortOrder, apiKey)
	if cached, ok := cache.Get[[]models.Paper](cacheK, 30*time.Minute); ok {
		return *cached, nil
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("fields", "title,authors,year,abstract,url,venue,externalIds,citationCount,publicationDate,openAccessPdf,journal")

	if yearFrom != "" && yearTo != "" {
		params.Set("year", fmt.Sprintf("%s-%s", yearFrom, yearTo))
	} else if yearFrom != "" {
		params.Set("year", fmt.Sprintf("%s-", yearFrom))
	} else if yearTo != "" {
		params.Set("year", fmt.Sprintf("-%s", yearTo))
	}

	if sortOrder != "" && sortOrder != "relevance" {
		if sortOrder == "citations" {
			params.Set("sort", "citationCount:desc")
		} else if sortOrder == "date" {
			params.Set("sort", "publicationDate:desc")
		}
	}

	reqURL := fmt.Sprintf("%s/paper/search?%s", semanticBase, params.Encode())
	headers := getSemanticHeaders(apiKey)
	body, err := client.SafeGet(reqURL, headers, 3)
	if err != nil {
		return nil, err
	}

	var resp semanticSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var results []models.Paper
	for _, item := range resp.Data {
		results = append(results, parseSemanticPaper(item))
	}

	_ = cache.Set(cacheK, results)
	return results, nil
}

func GetSemanticPaper(identifier string, idType string, apiKey string) (*models.Paper, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no API key")
	}

	cacheK := cache.Key("semantic_paper", identifier, idType, apiKey)
	if cached, ok := cache.Get[models.Paper](cacheK, time.Hour); ok {
		return cached, nil
	}

	params := url.Values{}
	params.Set("fields", "title,authors,year,abstract,url,venue,externalIds,citationCount,publicationDate,openAccessPdf,journal,referenceCount,tldr")

	reqURL := fmt.Sprintf("%s/paper/%s:%s?%s", semanticBase, url.PathEscape(idType), url.PathEscape(identifier), params.Encode())
	headers := getSemanticHeaders(apiKey)
	body, err := client.SafeGet(reqURL, headers, 3)
	if err != nil {
		return nil, err
	}

	var p semanticPaper
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	if p.Title == "" {
		return nil, fmt.Errorf("paper not found on Semantic Scholar")
	}

	paper := parseSemanticPaper(p)
	_ = cache.Set(cacheK, paper)
	return &paper, nil
}
