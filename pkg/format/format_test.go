package format

import (
	"strings"
	"testing"

	"github.com/hossam1522/VerifiSci/pkg/models"
)

func TestFormatters(t *testing.T) {
	paper := models.Paper{
		Title:     "Attention Is All You Need",
		Authors:   []string{"Ashish Vaswani", "Noam Shazeer"},
		Year:      "2017",
		DOI:       "10.1234/test.doi",
		URL:       "https://doi.org/10.1234/test.doi",
		Venue:     "NeurIPS",
		Journal:   "Advances in Neural Information Processing Systems",
		Volume:    "30",
		Pages:     "5998-6008",
		BibTeXKey: "Vaswani2017attention",
	}

	bib := BibTeX(paper)
	if !strings.Contains(bib, "@article{Vaswani2017attention,") || !strings.Contains(bib, "title = {Attention Is All You Need}") {
		t.Errorf("BibTeX() produced unexpected output:\n%s", bib)
	}

	apa := APA(paper)
	if !strings.Contains(apa, "Ashish Vaswani, Noam Shazeer (2017). Attention Is All You Need.") {
		t.Errorf("APA() produced unexpected output:\n%s", apa)
	}

	mla := MLA(paper)
	if !strings.Contains(mla, `Ashish Vaswani. "Attention Is All You Need."`) {
		t.Errorf("MLA() produced unexpected output:\n%s", mla)
	}
}
