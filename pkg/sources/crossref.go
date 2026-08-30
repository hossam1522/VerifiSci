package sources

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hossam1522/VerifiSci/pkg/cache"
	"github.com/hossam1522/VerifiSci/pkg/client"
	"github.com/hossam1522/VerifiSci/pkg/models"
)

const crossrefBase = "https://api.crossref.org/works"

type crossrefResponse struct {
	Message struct {
		Items []crossrefWork `json:"items"`
	} `json:"message"`
}

type crossrefSingleResponse struct {
	Message crossrefWork `json:"message"`
}

type crossrefWork struct {
	Title             []string          `json:"title"`
	Author            []crossrefAuthor  `json:"author"`
	DOI               string            `json:"DOI"`
	Publisher         string            `json:"publisher"`
	ContainerTitle    []string          `json:"container-title"`
	IsReferencedBy    int               `json:"is-referenced-by-count"`
	Volume            string            `json:"volume"`
	Issue             string            `json:"issue"`
	Page              string            `json:"page"`
	Abstract          string            `json:"abstract"`
	PublishedPrint    *crossrefDate     `json:"published-print"`
	PublishedOnline   *crossrefDate     `json:"published-online"`
}

type crossrefAuthor struct {
	Given  string `json:"given"`
	Family string `json:"family"`
}

type crossrefDate struct {
	DateParts [][]int `json:"date-parts"`
}

func parseCrossRefWork(item crossrefWork) models.Paper {
	var authors []string
	for _, a := range item.Author {
		name := strings.TrimSpace(fmt.Sprintf("%s %s", a.Given, a.Family))
		if name == "" {
			name = a.Family
		}
		if name != "" {
			authors = append(authors, name)
		}
	}

	title := ""
	if len(item.Title) > 0 {
		title = item.Title[0]
	}

	journal := ""
	if len(item.ContainerTitle) > 0 {
		journal = item.ContainerTitle[0]
	}

	yearStr := ""
	dateObj := item.PublishedPrint
	if dateObj == nil {
		dateObj = item.PublishedOnline
	}
	if dateObj != nil && len(dateObj.DateParts) > 0 && len(dateObj.DateParts[0]) > 0 {
		yearStr = strconv.Itoa(dateObj.DateParts[0][0])
	}

	paperURL := ""
	if item.DOI != "" {
		paperURL = fmt.Sprintf("https://doi.org/%s", item.DOI)
	}

	return models.Paper{
		Title:       title,
		Authors:     authors,
		Year:        yearStr,
		DOI:         item.DOI,
		URL:         paperURL,
		Venue:       item.Publisher,
		Journal:     journal,
		Volume:      item.Volume,
		Issue:       item.Issue,
		Pages:       item.Page,
		Publisher:   item.Publisher,
		Citations:   item.IsReferencedBy,
		Abstract:    item.Abstract,
		Source:      "crossref",
		BibTeXKey:   models.MakeBibTeXKey(authors, yearStr, title),
		Type:        "article",
	}
}

func SearchCrossRef(query string, limit int, yearFrom string, yearTo string, sortOrder string) ([]models.Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	cacheK := cache.Key("crossref_search", query, strconv.Itoa(limit), yearFrom, yearTo, sortOrder)
	if cached, ok := cache.Get[[]models.Paper](cacheK, time.Hour); ok {
		return *cached, nil
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("rows", strconv.Itoa(limit))
	if sortOrder != "" && sortOrder != "relevance" {
		if sortOrder == "citations" {
			params.Set("sort", "is-referenced-by-count")
		} else if sortOrder == "date" {
			params.Set("sort", "published")
		}
	}

	var filterParts []string
	if yearFrom != "" {
		filterParts = append(filterParts, fmt.Sprintf("from-pub-date:%s-01-01", yearFrom))
	}
	if yearTo != "" {
		filterParts = append(filterParts, fmt.Sprintf("until-pub-date:%s-12-31", yearTo))
	}
	if len(filterParts) > 0 {
		params.Set("filter", strings.Join(filterParts, ","))
	}

	reqURL := fmt.Sprintf("%s?%s", crossrefBase, params.Encode())
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var resp crossrefResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var results []models.Paper
	for _, item := range resp.Message.Items {
		results = append(results, parseCrossRefWork(item))
	}

	_ = cache.Set(cacheK, results)
	return results, nil
}

func GetCrossRefPaper(doi string) (*models.Paper, error) {
	cacheK := cache.Key("crossref_paper", doi)
	if cached, ok := cache.Get[models.Paper](cacheK, time.Hour); ok {
		return cached, nil
	}

	reqURL := fmt.Sprintf("%s/%s", crossrefBase, url.PathEscape(doi))
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var resp crossrefSingleResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Message.Title) == 0 {
		return nil, fmt.Errorf("paper not found")
	}

	paper := parseCrossRefWork(resp.Message)
	_ = cache.Set(cacheK, paper)
	return &paper, nil
}
