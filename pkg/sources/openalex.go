package sources

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hossam1522/VerifiSci/pkg/cache"
	"github.com/hossam1522/VerifiSci/pkg/client"
	"github.com/hossam1522/VerifiSci/pkg/models"
)

const openAlexBase = "https://api.openalex.org/works"

type openAlexResponse struct {
	Results []openAlexWork `json:"results"`
}

type openAlexWork struct {
	ID                     string                 `json:"id"`
	DOI                    string                 `json:"doi"`
	Title                  string                 `json:"title"`
	PublicationYear        int                    `json:"publication_year"`
	CitedByCount           int                    `json:"cited_by_count"`
	Authorships            []openAlexAuthorship   `json:"authorships"`
	PrimaryLocation        *openAlexLocation      `json:"primary_location"`
	Biblio                 openAlexBiblio         `json:"biblio"`
	HostVenue              *openAlexHostVenue     `json:"host_venue"`
	AbstractInvertedIndex  map[string][]int       `json:"abstract_inverted_index"`
}

type openAlexAuthorship struct {
	Author struct {
		DisplayName string `json:"display_name"`
	} `json:"author"`
}

type openAlexLocation struct {
	PDFURL string          `json:"pdf_url"`
	Source *openAlexSource `json:"source"`
}

type openAlexSource struct {
	DisplayName string `json:"display_name"`
	HostOrg     string `json:"host_organization_name"`
}

type openAlexBiblio struct {
	Volume    string `json:"volume"`
	Issue     string `json:"issue"`
	FirstPage string `json:"first_page"`
	LastPage  string `json:"last_page"`
}

type openAlexHostVenue struct {
	Publisher string `json:"publisher"`
}

func reconstructAbstract(invertedIndex map[string][]int) string {
	if len(invertedIndex) == 0 {
		return ""
	}
	type wordPos struct {
		word string
		pos  int
	}
	var pairs []wordPos
	for w, positions := range invertedIndex {
		for _, pos := range positions {
			pairs = append(pairs, wordPos{word: w, pos: pos})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].pos < pairs[j].pos
	})
	words := make([]string, len(pairs))
	for i, p := range pairs {
		words[i] = p.word
	}
	return strings.Join(words, " ")
}

func parseOpenAlexWork(item openAlexWork) models.Paper {
	var authors []string
	for _, a := range item.Authorships {
		if a.Author.DisplayName != "" {
			authors = append(authors, a.Author.DisplayName)
		}
	}

	doi := strings.TrimPrefix(item.DOI, "https://doi.org/")
	yearStr := ""
	if item.PublicationYear > 0 {
		yearStr = strconv.Itoa(item.PublicationYear)
	}

	venue := ""
	pdfURL := ""
	if item.PrimaryLocation != nil {
		pdfURL = item.PrimaryLocation.PDFURL
		if item.PrimaryLocation.Source != nil {
			venue = item.PrimaryLocation.Source.DisplayName
		}
	}

	publisher := ""
	if item.HostVenue != nil && item.HostVenue.Publisher != "" {
		publisher = item.HostVenue.Publisher
	} else if item.PrimaryLocation != nil && item.PrimaryLocation.Source != nil {
		publisher = item.PrimaryLocation.Source.HostOrg
	}

	pages := ""
	if item.Biblio.FirstPage != "" || item.Biblio.LastPage != "" {
		pages = strings.Trim(fmt.Sprintf("%s-%s", item.Biblio.FirstPage, item.Biblio.LastPage), "-")
	}

	paperURL := item.DOI
	if paperURL == "" {
		paperURL = item.ID
	}

	abstract := reconstructAbstract(item.AbstractInvertedIndex)

	return models.Paper{
		Title:       item.Title,
		Authors:     authors,
		Year:        yearStr,
		DOI:         doi,
		URL:         paperURL,
		Venue:       venue,
		Journal:     venue,
		Volume:      item.Biblio.Volume,
		Issue:       item.Biblio.Issue,
		Pages:       pages,
		Publisher:   publisher,
		Citations:   item.CitedByCount,
		Abstract:    abstract,
		Source:      "openalex",
		PDFURL:      pdfURL,
		BibTeXKey:   models.MakeBibTeXKey(authors, yearStr, item.Title),
		Type:        "article",
	}
}

func SearchOpenAlex(query string, limit int, yearFrom string, yearTo string, sortOrder string) ([]models.Paper, error) {
	if limit <= 0 {
		limit = 10
	}
	cacheK := cache.Key("openalex_search", query, strconv.Itoa(limit), yearFrom, yearTo, sortOrder)
	if cached, ok := cache.Get[[]models.Paper](cacheK, time.Hour); ok {
		return *cached, nil
	}

	sortMap := map[string]string{
		"relevance": "relevance_score:desc",
		"citations": "cited_by_count:desc",
		"date":      "publication_date:desc",
	}
	oaSort, ok := sortMap[sortOrder]
	if !ok {
		oaSort = "relevance_score:desc"
	}

	params := url.Values{}
	params.Set("search", query)
	params.Set("per_page", strconv.Itoa(limit))
	params.Set("sort", oaSort)

	var filterParts []string
	if yearFrom != "" {
		if y, err := strconv.Atoi(yearFrom); err == nil {
			filterParts = append(filterParts, fmt.Sprintf("publication_year:>%d", y-1))
		}
	}
	if yearTo != "" {
		if y, err := strconv.Atoi(yearTo); err == nil {
			filterParts = append(filterParts, fmt.Sprintf("publication_year:<%d", y+1))
		}
	}
	if len(filterParts) > 0 {
		params.Set("filter", strings.Join(filterParts, ","))
	}

	reqURL := fmt.Sprintf("%s?%s", openAlexBase, params.Encode())
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var resp openAlexResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var results []models.Paper
	for _, item := range resp.Results {
		results = append(results, parseOpenAlexWork(item))
	}

	_ = cache.Set(cacheK, results)
	return results, nil
}

func GetOpenAlexPaper(identifier string) (*models.Paper, error) {
	cacheK := cache.Key("openalex_paper", identifier)
	if cached, ok := cache.Get[models.Paper](cacheK, time.Hour); ok {
		return cached, nil
	}

	oaID := identifier
	if !strings.HasPrefix(oaID, "http") {
		oaID = "https://doi.org/" + identifier
	}

	reqURL := fmt.Sprintf("%s/%s", openAlexBase, oaID)
	body, err := client.SafeGet(reqURL, nil, 3)
	if err != nil {
		return nil, err
	}

	var work openAlexWork
	if err := json.Unmarshal(body, &work); err != nil {
		return nil, err
	}
	if work.Title == "" {
		return nil, fmt.Errorf("paper not found")
	}

	paper := parseOpenAlexWork(work)
	_ = cache.Set(cacheK, paper)
	return &paper, nil
}
