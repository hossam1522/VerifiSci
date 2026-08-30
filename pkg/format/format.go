package format

import (
	"fmt"
	"strings"

	"github.com/hossam1522/VerifiSci/pkg/models"
)

func BibTeX(paper models.Paper) string {
	citeKey := paper.BibTeXKey
	if citeKey == "" {
		citeKey = models.MakeBibTeXKey(paper.Authors, paper.Year, paper.Title)
	}

	eType := paper.Type
	if eType == "" {
		eType = "article"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("@%s{%s,\n", eType, citeKey))

	fields := []struct {
		k, v string
	}{
		{"title", paper.Title},
		{"author", strings.Join(paper.Authors, " and ")},
		{"journal", func() string {
			if paper.Journal != "" {
				return paper.Journal
			}
			return paper.Venue
		}()},
		{"year", paper.Year},
		{"volume", paper.Volume},
		{"number", paper.Issue},
		{"pages", paper.Pages},
		{"publisher", paper.Publisher},
		{"doi", paper.DOI},
		{"url", paper.URL},
	}

	for _, f := range fields {
		if f.v != "" {
			sb.WriteString(fmt.Sprintf("  %s = {%s},\n", f.k, f.v))
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func APA(paper models.Paper) string {
	authors := strings.Join(paper.Authors, ", ")
	year := paper.Year
	if year == "" {
		year = "(n.d.)"
	}
	title := paper.Title
	journal := paper.Journal
	if journal == "" {
		journal = paper.Venue
	}
	apa := fmt.Sprintf("%s (%s). %s.", authors, year, title)
	if journal != "" {
		apa += " " + journal
		if paper.Volume != "" {
			apa += fmt.Sprintf(", %s", paper.Volume)
			if paper.Pages != "" {
				apa += fmt.Sprintf(", %s", paper.Pages)
			}
		}
		apa += "."
	}
	if paper.DOI != "" {
		apa += fmt.Sprintf(" https://doi.org/%s", paper.DOI)
	}
	return apa
}

func MLA(paper models.Paper) string {
	firstAuthor := ""
	if len(paper.Authors) > 0 {
		firstAuthor = paper.Authors[0]
	}
	title := paper.Title
	journal := paper.Journal
	if journal == "" {
		journal = paper.Venue
	}
	year := paper.Year
	mla := fmt.Sprintf(`%s. "%s." %s (%s).`, firstAuthor, title, journal, year)
	if paper.DOI != "" {
		mla += fmt.Sprintf(" doi:%s.", paper.DOI)
	}
	return mla
}
