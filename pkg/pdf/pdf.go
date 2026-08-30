package pdf

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

var (
	exitKeywords = []string{
		"references", "bibliography", "acknowledgments", "acknowledgements",
		"appendix", "appendices", "author contributions", "competing interests",
		"funding", "data availability", "code availability", "declarations",
	}

	exitPattern = regexp.MustCompile(
		`^(?i)(?:(?:section\s+)?(?:\d+|[ivxlcdm]+)(?:\.\d+)*[\.:\)]?\s+)?(` +
			strings.Join(exitKeywords, "|") +
			`)(?:\s*[:\-–—\.]|\s*$)`,
	)

	inlineAckPattern = regexp.MustCompile(`(?i)\b(?:acknowledg(?:e)?ments?|references|bibliography)\b`)
	nextSecPattern   = regexp.MustCompile(`^\d+\s+[A-Z][a-zA-Z\s]{2,40}$`)
)

func DownloadAndExtractPDFText(url string, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 150000
	}

	client := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "VerifiSci/1.0 (https://github.com/hossam1522/VerifiSci; mailto:verifisci@example.com)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PDF download returned status %d", resp.StatusCode)
	}

	pdfData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Method 1: Try pdftotext CLI if installed on the system (very accurate font spacing)
	if pdftotextPath, err := exec.LookPath("pdftotext"); err == nil {
		tmpFile, err := os.CreateTemp("", "verifisci-*.pdf")
		if err == nil {
			tmpName := tmpFile.Name()
			_, _ = tmpFile.Write(pdfData)
			_ = tmpFile.Close()
			defer os.Remove(tmpName)

			cmd := exec.Command(pdftotextPath, "-l", "50", tmpName, "-")
			var out bytes.Buffer
			cmd.Stdout = &out
			if cmd.Run() == nil && out.Len() > 0 {
				res := out.String()
				if len(res) > maxChars {
					res = res[:maxChars]
				}
				return res, nil
			}
		}
	}

	// Method 2: Pure Go PDF extractor using github.com/ledongthuc/pdf
	reader, err := pdf.NewReader(bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	numPages := reader.NumPage()
	if numPages > 50 {
		numPages = 50
	}

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		p := reader.Page(pageNum)
		if p.V.IsNull() {
			continue
		}

		// Try row-based extraction to preserve whitespace
		rows, err := p.GetTextByRow()
		if err == nil && len(rows) > 0 {
			for _, row := range rows {
				var line strings.Builder
				for _, word := range row.Content {
					line.WriteString(word.S)
					line.WriteString(" ")
				}
				sb.WriteString(strings.TrimSpace(line.String()))
				sb.WriteString("\n")
			}
		} else {
			// Fallback to page plain text
			text, err := p.GetPlainText(nil)
			if err == nil {
				sb.WriteString(text)
			}
		}

		sb.WriteString("\n\n")
		if sb.Len() > maxChars {
			break
		}
	}

	text := sb.String()
	if len(text) > maxChars {
		text = text[:maxChars]
	}
	return text, nil
}

func ExtractSection(text string, sectionNames []string, maxChars int) string {
	if text == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 5000
	}

	escapedNames := make([]string, len(sectionNames))
	for i, s := range sectionNames {
		escapedNames[i] = regexp.QuoteMeta(s)
	}

	headerPattern := regexp.MustCompile(
		`^(?i)(?:(?:section\s+)?(?:\d+|[ivxlcdm]+)(?:\.\d+)*[\.:\)]?\s+)?(` +
			strings.Join(escapedNames, "|") +
			`)(?:\s*[:\-–—\.]|\s+and\s+.*|\s+remarks|\s*$)`,
	)

	lines := strings.Split(text, "\n")
	inSection := false
	var collectedLines []string
	chars := 0

	for _, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			if inSection && len(collectedLines) > 0 && collectedLines[len(collectedLines)-1] != "" {
				collectedLines = append(collectedLines, "")
			}
			continue
		}

		if !inSection {
			if len(stripped) < 80 && headerPattern.MatchString(stripped) {
				inSection = true
				continue
			}
		}

		if inSection {
			if len(stripped) < 80 && (exitPattern.MatchString(stripped) || nextSecPattern.MatchString(stripped)) {
				break
			}

			if loc := inlineAckPattern.FindStringIndex(stripped); loc != nil {
				prefix := strings.TrimSpace(stripped[:loc[0]])
				if prefix != "" {
					collectedLines = append(collectedLines, prefix)
				}
				break
			}

			collectedLines = append(collectedLines, stripped)
			chars += len(stripped)
			if chars > maxChars {
				collectedLines = append(collectedLines, "[...truncated...]")
				break
			}
		}
	}

	if len(collectedLines) == 0 {
		return ""
	}

	var paragraphs []string
	var currentP []string
	for _, l := range collectedLines {
		if l == "" {
			if len(currentP) > 0 {
				paragraphs = append(paragraphs, strings.Join(currentP, " "))
				currentP = nil
			}
		} else if l == "[...truncated...]" {
			if len(currentP) > 0 {
				paragraphs = append(paragraphs, strings.Join(currentP, " "))
				currentP = nil
			}
			paragraphs = append(paragraphs, l)
		} else {
			currentP = append(currentP, l)
		}
	}
	if len(currentP) > 0 {
		paragraphs = append(paragraphs, strings.Join(currentP, " "))
	}

	return strings.Join(paragraphs, "\n\n")
}
