# VerifiSci

Fast, standalone CLI tool for academic paper search and citation generation. Built in Go with zero runtime dependencies. Designed for LLM agents and researchers to find and cite reliable academic sources without hallucination.

## Installation

### Option 1: Download Pre-compiled Binary (Recommended)
Download the latest binary for your OS and architecture from [GitHub Releases](https://github.com/hossam1522/VerifiSci/releases):

```bash
# Linux (x86_64)
curl -sSL https://github.com/hossam1522/VerifiSci/releases/latest/download/verifisci-linux-amd64.tar.gz | tar -xz
sudo mv verifisci-linux-amd64 /usr/local/bin/verifisci

# macOS (Apple Silicon M1-M4)
curl -sSL https://github.com/hossam1522/VerifiSci/releases/latest/download/verifisci-darwin-arm64.tar.gz | tar -xz
sudo mv verifisci-darwin-arm64 /usr/local/bin/verifisci
```

### Option 2: Using `go install`
```bash
go install github.com/hossam1522/VerifiSci@latest
```

### Option 3: Build from Source
```bash
git clone https://github.com/hossam1522/VerifiSci.git
cd VerifiSci
make build
# Binary is built at ./bin/verifisci
```

## Quick Start

```bash
# Search papers (JSON output by default for LLMs)
verifisci search "graph neural networks" --limit 5

# Human-readable output
verifisci search "transformer architecture" --text
```

## Commands

### `search` — Find academic papers

```bash
# Basic search (default source: openalex, no API key needed)
verifisci search "your query here" --limit 10

# Search all available free sources
verifisci search "attention mechanism" --source all --limit 15

# Filter by year
verifisci search "large language models" --year-from 2020 --year-to 2024

# Sort by citations (most cited first)
verifisci search "reinforcement learning" --sort citations

# Search arXiv preprints only
verifisci search "diffusion models" --source arxiv

# Human-readable output (instead of JSON)
verifisci search "transformer architecture" --text

# BibTeX output for all results
verifisci search "graph neural networks" --bibtex
```

**Options:**

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--source`, `-s` | `openalex`, `crossref`, `arxiv`, `semantic`, `google`, `all` | `openalex` | Data source |
| `--limit`, `-n` | integer | `10` | Max results |
| `--year-from` | YYYY | — | Filter from year |
| `--year-to` | YYYY | — | Filter to year |
| `--sort` | `relevance`, `citations`, `date` | `relevance` | Sort order |
| `--text`, `-t` | flag | — | Human-readable output |
| `--bibtex`, `-b` | flag | — | BibTeX-formatted output |
| `--json`, `-j` | flag | — | JSON output (default) |
| `--semantic-key` | string | — | Semantic Scholar API key |
| `--proxy` | flag | — | Use proxy for Google Scholar |

### `cite` — Generate citations from a paper identifier

```bash
# BibTeX (default)
verifisci cite 10.1038/nature14539

# APA format
verifisci cite 10.1038/nature14539 --format apa

# MLA format
verifisci cite 10.1038/nature14539 --format mla

# From arXiv ID
verifisci cite 1706.03762 --type ARXIV

# From URL
verifisci cite "https://arxiv.org/abs/1706.03762" --type URL
```

### `get` — Get full paper details

```bash
# JSON output (default)
verifisci get 10.1038/nature14539

# Human-readable
verifisci get 10.1038/nature14539 --text

# BibTeX for a specific paper
verifisci get 10.1038/nature14539 --bibtex
```

### `read` — Read paper content (abstract, conclusions, full text)

This is the critical command for LLM agents writing articles: it extracts what the paper actually says so the agent can cite it accurately without hallucinating.

```bash
# Read an arXiv paper (gets metadata + abstract + full text + conclusions)
verifisci read https://arxiv.org/abs/1706.03762 --type URL

# Read by arXiv ID
verifisci read 1706.03762 --type ARXIV

# Read ONLY abstract and conclusions (ideal for LLM agents to save context tokens)
verifisci read 1706.03762 --type ARXIV --no-full-text --text

# Read only abstract or only conclusions
verifisci read 1706.03762 --type ARXIV --abstract-only --text
verifisci read 1706.03762 --type ARXIV --conclusions-only --text

# Read entire paper with unlimited characters
verifisci read 1706.03762 --type ARXIV --max-chars -1 --text

# Read a paper by DOI (gets metadata + abstract)
verifisci read 10.1038/nature14539

# Control character limit of extracted text
verifisci read 1706.03762 --type ARXIV --max-chars 10000

# Human-readable output (shows abstract, conclusions, and full text sections)
verifisci read 1706.03762 --type ARXIV --text
```

**Options for `read`:**

| Flag | Description |
|------|-------------|
| `--no-full-text`, `--summary` | Omits the full text entirely. Only returns metadata, abstract, and conclusions (great for minimizing LLM context tokens). |
| `--abstract-only` | Only returns metadata and abstract. |
| `--conclusions-only` | Only returns metadata and conclusions. |
| `--max-chars N` | Maximum characters of full text (default: 20000, `0` to omit, `-1` for unlimited). |
| `--text`, `-t` | Output in clean, readable text format. |
| `--json`, `-j` | Output as JSON (default). |

**What `read` extracts:**
- **Metadata**: title, authors, year, DOI, URL, citations
- **Abstract**: full paper abstract
- **Conclusions**: the conclusion/discussion section automatically extracted from the PDF
- **Full text**: full paper text (or truncated at `--max-chars`, omitted when `--no-full-text` is used)

**Note on paywalled papers:** For papers behind paywalls (e.g., Nature, IEEE), `read` will get metadata and abstract but cannot access the full text. arXiv papers are always fully accessible.

## JSON Output Format

The default JSON output includes all fields an LLM agent needs:

```json
{
  "query": "graph neural networks",
  "total_results": 5,
  "results": [
    {
      "title": "The Graph Neural Network Model",
      "authors": ["Franco Scarselli", "M. Gori", "Ah Chung Tsoi"],
      "year": "2008",
      "doi": "10.1109/tnn.2008.2005605",
      "url": "https://doi.org/10.1109/tnn.2008.2005605",
      "venue": "IEEE Transactions on Neural Networks",
      "citations": 9272,
      "abstract": "Many underlying relationships among data...",
      "source": "openalex",
      "pdf_url": null,
      "bibtex_key": "Scarselli2008the",
      "type": "article",
      "journal": "IEEE Transactions on Neural Networks",
      "volume": "20",
      "issue": "1",
      "pages": "61-80",
      "publisher": "Institute of Electrical and Electronics Engineers",
      "arxiv_id": ""
    }
  ],
  "sources_used": ["openalex"]
}
```

## Data Sources

### OpenAlex (default, recommended)
- **No API key required**
- Free and open, comprehensive index of ~250M works
- Best overall relevance for academic queries
- Rate limit: ~10 requests/second

### arXiv
- **No API key required**
- Preprints in physics, CS, math, statistics
- Full-text PDF available for most papers

### CrossRef
- **No API key required**
- DOI registry, good for citation metadata
- Best for BibTeX generation from DOIs

### Semantic Scholar
- **API key recommended** (free at [semanticscholar.org](https://www.semanticscholar.org/product/api#api-key-form))
- Set env var: `export SEMANTIC_SCHOLAR_API_KEY=your_key_here`
- Or pass via CLI: `--semantic-key YOUR_KEY`
- Without key: very limited rate limits
- **Note on new keys:** even after the "your key has been approved" email, Semantic
  Scholar's own docs warn it can take up to 24h for the key to actually propagate to
  their auth backend — a fresh key returning 429 for the first few hours is expected,
  not a sign it's misconfigured.
- **HTTP/2 is required**, not just the key. Semantic Scholar's edge (CloudFront) 429s
  requests made over HTTP/1.1 — which is what `requests`/plain `curl` speak by
  default — even with a valid, active key and correct `x-api-key` header. It only lets
  HTTP/2 requests through reliably. This is why Semantic Scholar calls in this codebase
  go through `httpx` (`safe_get_http2`/`HTTP2_CLIENT`, `http2=True`) instead of the
  `requests`-based `safe_get` used for the other sources. If you add another Semantic
  Scholar endpoint, reuse `safe_get_http2`, not `safe_get` — the latter will look like
  it's failing due to rate limits when the real cause is the protocol.
- The `fields` query parameter only accepts Semantic Scholar's documented top-level
  fields. `volume` and `pages` are **not** top-level fields (the API returns a `400`
  for them) — they exist only nested inside `journal` (`journal.volume`,
  `journal.pages`), which is already requested via the `journal` field.

### Google Scholar
- **No API key** (but uses scraping, may get rate-limited)
- Use `--proxy` flag to attempt proxy rotation
- Best for citation counts and gray literature

## Cache

Results are cached in `~/.cache/verifisci/` for 30-60 minutes to reduce API calls. Clear with:

```bash
rm -rf ~/.cache/verifisci
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SEMANTIC_SCHOLAR_API_KEY` | API key for Semantic Scholar (higher rate limits) |

## For LLM Agent Usage

The tool is designed to be called from an LLM agent workflow:

1. **Find sources:** `verifisci search "topic" --limit 10 --json`
2. **Read the paper:** `verifisci read ARXIV_ID_OR_DOI --text` — extracts abstract, conclusions, and full text so the agent knows what the paper actually says
3. **Get details:** `verifisci get DOI --json`
4. **Generate citation:** `verifisci cite DOI --format bibtex`
5. **Insert into document:** Use the BibTeX key and citation in your LaTeX/Markdown

Example agent workflow:
```bash
# Agent searches for references
verifisci search "transformer attention mechanism" --limit 5 --json

# Agent reads the most relevant paper to understand it
verifisci read 1706.03762 --type ARXIV --text

# Agent now knows: what the paper argues, its conclusions, methodology
# and can cite it properly without hallucinating

# Generate the BibTeX citation for the paper
python verifisci.py cite 1706.03762 --type ARXIV
```

### Why `read` is important for LLM agents

LLMs hallucinate citations. They often:
- Invent plausible-sounding but non-existent papers
- Misattribute findings to the wrong paper
- Summarize a paper's conclusions incorrectly based on the title alone

The `read` command gives the agent access to the actual paper content:
- **Abstract** — what the paper claims to contribute
- **Conclusions** — what the paper actually found
- **Full text** (arXiv only) — methodology, experiments, results

This lets the agent write about papers with real understanding, not just guesswork.

## Notes

- OpenAlex is the default because it requires no API key and provides excellent results
- The `--source all` option interleaves results from multiple sources and deduplicates by DOI/title
- Semantic Scholar usually has the best relevance but needs an API key for reliable access
- Google Scholar scraping can be unreliable; prefer OpenAlex or Semantic Scholar when possible
