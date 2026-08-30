package models

import "testing"

func TestMakeBibTeXKey(t *testing.T) {
	tests := []struct {
		authors []string
		year    string
		title   string
		want    string
	}{
		{
			authors: []string{"Ashish Vaswani", "Noam Shazeer"},
			year:    "2017",
			title:   "Attention Is All You Need",
			want:    "Vaswani2017attention",
		},
		{
			authors: []string{"Yann LeCun"},
			year:    "2015",
			title:   "Deep learning",
			want:    "LeCun2015deep",
		},
		{
			authors: nil,
			year:    "",
			title:   "",
			want:    "unknown????x",
		},
	}

	for _, tt := range tests {
		got := MakeBibTeXKey(tt.authors, tt.year, tt.title)
		if got != tt.want {
			t.Errorf("MakeBibTeXKey(%v, %q, %q) = %q; want %q", tt.authors, tt.year, tt.title, got, tt.want)
		}
	}
}
