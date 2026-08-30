package sources

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hossam1522/VerifiSci/pkg/models"
	"github.com/hossam1522/VerifiSci/pkg/pdf"
)

func SearchAll(query string, limit int, yearFrom string, yearTo string, sortOrder string, sourcesList []string, semanticKey string) (*models.SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if len(sourcesList) == 0 {
		sourcesList = []string{"openalex", "crossref", "arxiv"}
	}

	type sourceResult struct {
		source string
		papers []models.Paper
	}

	resultsChan := make(chan sourceResult, len(sourcesList))
	var wg sync.WaitGroup

	fetchLimit := limit
	if fetchLimit < 5 {
		fetchLimit = 5
	}

	for _, src := range sourcesList {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			var papers []models.Paper
			var err error

			switch strings.ToLower(s) {
			case "openalex":
				papers, err = SearchOpenAlex(query, fetchLimit, yearFrom, yearTo, sortOrder)
			case "crossref":
				papers, err = SearchCrossRef(query, fetchLimit, yearFrom, yearTo, sortOrder)
			case "arxiv":
				papers, err = SearchArXiv(query, fetchLimit, semanticKey)
			case "semantic":
				papers, err = SearchSemanticScholar(query, fetchLimit, yearFrom, yearTo, sortOrder, semanticKey)
			}

			if err == nil && len(papers) > 0 {
				resultsChan <- sourceResult{source: s, papers: papers}
			}
		}(src)
	}

	wg.Wait()
	close(resultsChan)

	sourceMap := make(map[string][]models.Paper)
	var maxLen int
	for sr := range resultsChan {
		sourceMap[sr.source] = sr.papers
		if len(sr.papers) > maxLen {
			maxLen = len(sr.papers)
		}
	}

	seen := make(map[string]bool)
	var interleaved []models.Paper

	for i := 0; i < maxLen; i++ {
		for _, src := range sourcesList {
			list := sourceMap[src]
			if i < len(list) {
				p := list[i]
				key := strings.ToLower(strings.TrimSpace(p.DOI))
				if key == "" {
					key = normalizeString(p.Title)
				}
				if key != "" && !seen[key] {
					seen[key] = true
					interleaved = append(interleaved, p)
				}
			}
		}
	}

	if sortOrder == "citations" {
		sort.SliceStable(interleaved, func(i, j int) bool {
			return interleaved[i].Citations > interleaved[j].Citations
		})
	} else if sortOrder == "date" {
		sort.SliceStable(interleaved, func(i, j int) bool {
			return interleaved[i].Year > interleaved[j].Year
		})
	}

	if len(interleaved) > limit {
		interleaved = interleaved[:limit]
	}

	return &models.SearchResult{
		Query:        query,
		TotalResults: len(interleaved),
		Results:      interleaved,
		SourcesUsed:  sourcesList,
	}, nil
}

func ExtractIdentifierInfo(identifier string, idType string) (resolvedID string, resolvedType string) {
	idType = strings.ToUpper(strings.TrimSpace(idType))
	identifier = strings.TrimSpace(identifier)

	if strings.Contains(identifier, "arxiv.org/abs/") {
		parts := strings.Split(identifier, "arxiv.org/abs/")
		return strings.Trim(parts[len(parts)-1], "/.pdf"), "ARXIV"
	}
	if strings.Contains(identifier, "arxiv.org/pdf/") {
		parts := strings.Split(identifier, "arxiv.org/pdf/")
		return strings.Trim(parts[len(parts)-1], "/.pdf"), "ARXIV"
	}
	if strings.Contains(identifier, "doi.org/") {
		parts := strings.Split(identifier, "doi.org/")
		return strings.Trim(parts[len(parts)-1], "/"), "DOI"
	}

	if idType == "" {
		idType = "DOI"
	}
	return identifier, idType
}

func GetPaperDetails(identifier string, idType string, semanticKey string) (*models.Paper, error) {
	id, kind := ExtractIdentifierInfo(identifier, idType)

	// 1. If arXiv
	if kind == "ARXIV" {
		return GetArXivPaper(id, semanticKey)
	}

	// 2. Try Semantic Scholar if key is present
	if semanticKey != "" {
		if p, err := GetSemanticPaper(id, kind, semanticKey); err == nil && p != nil {
			return p, nil
		}
	}

	// 3. Try OpenAlex
	if p, err := GetOpenAlexPaper(id); err == nil && p != nil {
		return p, nil
	}

	// 4. Try CrossRef
	if kind == "DOI" {
		if p, err := GetCrossRefPaper(id); err == nil && p != nil {
			return p, nil
		}
	}

	return nil, fmt.Errorf("could not find paper with %s: %s", kind, id)
}

func ReadPaper(identifier string, idType string, maxChars int, includeFullText bool, semanticKey string) (*models.ReadResult, error) {
	id, kind := ExtractIdentifierInfo(identifier, idType)

	result := &models.ReadResult{
		Identifier:   identifier,
		IDType:       kind,
		SourceOfText: "none",
	}

	paper, err := GetPaperDetails(identifier, idType, semanticKey)
	if err == nil && paper != nil {
		result.Metadata = paper
		result.Abstract = paper.Abstract
	} else {
		errStr := fmt.Sprintf("Metadata lookup error: %v", err)
		result.Error = &errStr
	}

	pdfURL := ""
	if kind == "ARXIV" {
		pdfURL = fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", id)
	} else if paper != nil && paper.PDFURL != "" {
		pdfURL = paper.PDFURL
	}

	if pdfURL != "" {
		fullPDFText, err := pdf.DownloadAndExtractPDFText(pdfURL, 150000)
		if err == nil && fullPDFText != "" {
			result.SourceOfText = "pdf"

			conclusionSections := []string{
				"conclusion", "conclusions", "concluding remarks",
				"discussion and conclusion", "summary and conclusion",
				"conclusions and future work", "summary", "discussion",
			}
			result.Conclusions = pdf.ExtractSection(fullPDFText, conclusionSections, 5000)

			if includeFullText && maxChars != 0 {
				if maxChars == -1 || len(fullPDFText) <= maxChars {
					result.FullText = fullPDFText
				} else {
					remaining := len(fullPDFText) - maxChars
					result.FullText = fmt.Sprintf(
						"%s\n\n[... Full text truncated at %d characters (%d chars omitted). Use --no-full-text for summary only, or --max-chars -1 for complete text ...]",
						fullPDFText[:maxChars], maxChars, remaining,
					)
				}
			}
		} else if err != nil {
			if result.Error == nil {
				errStr := fmt.Sprintf("PDF extraction failed: %v", err)
				result.Error = &errStr
			} else {
				errStr := fmt.Sprintf("%s; PDF extraction failed: %v", *result.Error, err)
				result.Error = &errStr
			}
		}
	}

	return result, nil
}
