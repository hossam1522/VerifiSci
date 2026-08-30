package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hossam1522/VerifiSci/pkg/sources"
	"github.com/spf13/cobra"
)

var (
	readType            string
	readMaxChars        int
	readNoFullText      bool
	readSummary         bool
	readAbstractOnly    bool
	readConclusionsOnly bool
	readJSON            bool
	readText            bool
)

var readCmd = &cobra.Command{
	Use:   "read <identifier>",
	Short: "Read paper content: metadata, abstract, conclusions, full text",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]

		includeFullText := !(readNoFullText || readSummary || readAbstractOnly || readConclusionsOnly || readMaxChars == 0)
		maxChars := readMaxChars
		if !includeFullText {
			maxChars = 0
		}

		result, err := sources.ReadPaper(identifier, readType, maxChars, includeFullText, semanticKeyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading paper: %v\n", err)
			os.Exit(1)
		}

		if readAbstractOnly {
			result.Conclusions = ""
			result.FullText = ""
		} else if readConclusionsOnly {
			result.Abstract = ""
			result.FullText = ""
		} else if !includeFullText {
			result.FullText = ""
		}

		if readText {
			md := result.Metadata
			fmt.Println(strings.Repeat("=", 70))
			if md != nil {
				fmt.Printf("TITLE: %s\n", md.Title)
				fmt.Printf("AUTHORS: %s\n", strings.Join(md.Authors, ", "))
				fmt.Printf("YEAR: %s | CITATIONS: %d\n", md.Year, md.Citations)
				fmt.Printf("DOI: %s\n", md.DOI)
				fmt.Printf("URL: %s\n", md.URL)
			} else {
				fmt.Printf("IDENTIFIER: %s (%s)\n", result.Identifier, result.IDType)
			}
			fmt.Println(strings.Repeat("=", 70))

			if result.Abstract != "" && !readConclusionsOnly {
				fmt.Printf("\n--- ABSTRACT ---\n")
				ab := result.Abstract
				if len(ab) > 2000 {
					ab = ab[:2000]
				}
				fmt.Println(ab)
			}

			if result.Conclusions != "" && !readAbstractOnly {
				fmt.Printf("\n--- CONCLUSIONS ---\n")
				fmt.Println(result.Conclusions)
			}

			if result.FullText != "" && includeFullText {
				fmt.Printf("\n--- FULL TEXT (source: %s) ---\n", result.SourceOfText)
				fmt.Println(result.FullText)
			}

			if result.Error != nil {
				fmt.Printf("\n[NOTE] %s\n", *result.Error)
			}
			return
		}

		// Default: JSON output
		outMap := *result
		if outMap.FullText != "" && len(outMap.FullText) > 5000 && !(readMaxChars == -1 || readMaxChars > 5000) {
			outMap.FullText = outMap.FullText[:5000] + " [...truncated in JSON, use --text or --max-chars -1 for more]"
		}

		out, err := json.MarshalIndent(outMap, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	},
}

func init() {
	readCmd.Flags().StringVarP(&readType, "type", "t", "DOI", "Identifier type (DOI, ARXIV, URL)")
	readCmd.Flags().IntVar(&readMaxChars, "max-chars", 20000, "Max characters of full text (default: 20000, -1 for unlimited, 0 to omit)")
	readCmd.Flags().BoolVar(&readNoFullText, "no-full-text", false, "Only output metadata, abstract, and conclusions (omits full text)")
	readCmd.Flags().BoolVar(&readSummary, "summary", false, "Alias for --no-full-text")
	readCmd.Flags().BoolVar(&readAbstractOnly, "abstract-only", false, "Only output metadata and abstract")
	readCmd.Flags().BoolVar(&readConclusionsOnly, "conclusions-only", false, "Only output metadata and conclusions")
	readCmd.Flags().BoolVarP(&readJSON, "json", "j", false, "Output as JSON (default)")
	readCmd.Flags().BoolVar(&readText, "text", false, "Output as readable text")
	rootCmd.AddCommand(readCmd)
}
