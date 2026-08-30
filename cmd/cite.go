package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hossam1522/VerifiSci/pkg/format"
	"github.com/hossam1522/VerifiSci/pkg/sources"
	"github.com/spf13/cobra"
)

var (
	citeType   string
	citeFormat string
)

var citeCmd = &cobra.Command{
	Use:   "cite <identifier>",
	Short: "Generate citation for a paper",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]

		paper, err := sources.GetPaperDetails(identifier, citeType, semanticKeyFlag)
		if err != nil || paper == nil {
			fmt.Fprintf(os.Stderr, "Could not find paper with %s: %s\n", citeType, identifier)
			os.Exit(1)
		}

		switch strings.ToLower(citeFormat) {
		case "apa":
			fmt.Println(format.APA(*paper))
		case "mla":
			fmt.Println(format.MLA(*paper))
		default:
			fmt.Println(format.BibTeX(*paper))
		}
	},
}

func init() {
	citeCmd.Flags().StringVarP(&citeType, "type", "t", "DOI", "Identifier type (DOI, ARXIV, URL, PMID, CORPUSID)")
	citeCmd.Flags().StringVarP(&citeFormat, "format", "f", "bibtex", "Citation format (bibtex, apa, mla)")
	rootCmd.AddCommand(citeCmd)
}
