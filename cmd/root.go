package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "v1.0.0"
	Commit    = "none"
	BuildDate = "unknown"

	semanticKeyFlag string
)

var rootCmd = &cobra.Command{
	Use:     "verifisci",
	Short:   "VerifiSci - CLI academic search and citation tool for LLM agents",
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate),
	Long: `VerifiSci is a CLI tool for searching academic sources and generating citations.
Designed for LLM agents to find reliable academic references for writing articles/theses.

Sources: Semantic Scholar, CrossRef, OpenAlex, arXiv`,
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
