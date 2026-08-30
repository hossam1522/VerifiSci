package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	semanticKeyFlag string
)

var rootCmd = &cobra.Command{
	Use:   "verifisci",
	Short: "VerifiSci - CLI academic search and citation tool for LLM agents",
	Long: `VerifiSci is a CLI tool for searching academic sources and generating citations.
Designed for LLM agents to find reliable academic references for writing articles/theses.

Sources: Semantic Scholar, Google Scholar, CrossRef, OpenAlex, arXiv`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&semanticKeyFlag, "semantic-key", os.Getenv("SEMANTIC_SCHOLAR_API_KEY"), "Semantic Scholar API key (or SEMANTIC_SCHOLAR_API_KEY env)")
}
