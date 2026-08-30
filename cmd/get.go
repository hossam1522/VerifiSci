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
	getType   string
	getJSON   bool
	getText   bool
	getBibTeX bool
)

var getCmd = &cobra.Command{
	Use:   "get <identifier>",
	Short: "Get full details of a paper",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]

		paper, err := sources.GetPaperDetails(identifier, getType, semanticKeyFlag)
		if err != nil || paper == nil {
			fmt.Fprintf(os.Stderr, "Could not find paper with %s: %s\n", getType, identifier)
			os.Exit(1)
		}

		if getBibTeX {
			fmt.Println(format.BibTeX(*paper))
			return
		}

		if getText {
			fmt.Printf("Title: %s\n", paper.Title)
			fmt.Printf("Authors: %s\n", strings.Join(paper.Authors, ", "))
			fmt.Printf("Year: %s | Citations: %d\n", paper.Year, paper.Citations)
			if paper.DOI != "" {
				fmt.Printf("DOI: %s\n", paper.DOI)
			}
			if paper.URL != "" {
				fmt.Printf("URL: %s\n", paper.URL)
			}
			if paper.Journal != "" {
				fmt.Printf("Journal: %s\n", paper.Journal)
			}
			if paper.Abstract != "" {
				ab := paper.Abstract
				if len(ab) > 500 {
					ab = ab[:500] + "..."
				}
				fmt.Printf("Abstract: %s\n", ab)
			}
			return
		}

		// Default: JSON output
		out, err := json.MarshalIndent(paper, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	},
}

func init() {
	getCmd.Flags().StringVarP(&getType, "type", "t", "DOI", "Identifier type (DOI, ARXIV, URL, PMID, CORPUSID)")
	getCmd.Flags().BoolVarP(&getJSON, "json", "j", false, "Output as JSON (default)")
	getCmd.Flags().BoolVar(&getText, "text", false, "Output as readable text")
	getCmd.Flags().BoolVarP(&getBibTeX, "bibtex", "b", false, "Output as BibTeX")
	rootCmd.AddCommand(getCmd)
}
