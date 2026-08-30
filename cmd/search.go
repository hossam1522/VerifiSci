package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hossam1522/VerifiSci/pkg/format"
	"github.com/hossam1522/VerifiSci/pkg/sources"
	"github.com/spf13/cobra"
)

var (
	searchSource   string
	searchLimit    int
	searchYearFrom string
	searchYearTo   string
	searchSort     string
	searchJSON     bool
	searchText     bool
	searchBibTeX   bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for academic papers",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]

		var sourceList []string
		if strings.ToLower(searchSource) == "all" {
			sourceList = []string{"openalex", "crossref", "arxiv"}
			if semanticKeyFlag != "" {
				sourceList = append(sourceList, "semantic")
			}
		} else {
			sourceList = []string{searchSource}
		}

		result, err := sources.SearchAll(query, searchLimit, searchYearFrom, searchYearTo, searchSort, sourceList, semanticKeyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
			os.Exit(1)
		}

		if searchBibTeX {
			for _, p := range result.Results {
				fmt.Println(format.BibTeX(p))
				fmt.Println()
			}
			return
		}

		if searchText {
			for i, p := range result.Results {
				fmt.Printf("%d. %s\n", i+1, p.Title)
				authorSummary := strings.Join(p.Authors, ", ")
				if len(p.Authors) > 5 {
					authorSummary = strings.Join(p.Authors[:5], ", ") + " et al."
				}
				fmt.Printf("   Authors: %s\n", authorSummary)
				fmt.Printf("   Year: %s | Citations: %d | Source: %s\n", p.Year, p.Citations, p.Source)
				if p.DOI != "" {
					fmt.Printf("   DOI: %s\n", p.DOI)
				}
				if p.URL != "" {
					fmt.Printf("   URL: %s\n", p.URL)
				}
				if p.PDFURL != "" {
					fmt.Printf("   PDF: %s\n", p.PDFURL)
				}
				if p.Abstract != "" {
					ab := strings.ReplaceAll(p.Abstract, "\n", " ")
					if len(ab) > 300 {
						ab = ab[:300] + "..."
					}
					fmt.Printf("   Abstract: %s\n", ab)
				}
				fmt.Println()
			}
			return
		}

		// Default: JSON output
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchSource, "source", "s", "openalex", "Source to search (openalex, crossref, arxiv, semantic, all)")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "Max results")
	searchCmd.Flags().StringVar(&searchYearFrom, "year-from", "", "Filter from year")
	searchCmd.Flags().StringVar(&searchYearTo, "year-to", "", "Filter to year")
	searchCmd.Flags().StringVar(&searchSort, "sort", "relevance", "Sort order (relevance, citations, date)")
	searchCmd.Flags().BoolVarP(&searchJSON, "json", "j", false, "Output as JSON (default)")
	searchCmd.Flags().BoolVarP(&searchText, "text", "t", false, "Output as readable text")
	searchCmd.Flags().BoolVarP(&searchBibTeX, "bibtex", "b", false, "Output as BibTeX")
	rootCmd.AddCommand(searchCmd)
}
