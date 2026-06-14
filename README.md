# VerifiSci

CLI tool for academic paper search and citation generation. Designed for LLM agents to find and cite reliable academic sources when writing articles, theses, or research papers — without manual intervention.

## Quick Start

```bash
# One-time setup
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# Search papers (JSON output by default)
python verifisci.py search "graph neural networks" --limit 5
```

## Commands

### `search` — Find academic papers

```bash
# Basic search (default source: openalex, no API key needed)
python verifisci.py search "your query here" --limit 10

# Search all available free sources
python verifisci.py search "attention mechanism" --source all --limit 15

# Filter by year
python verifisci.py search "large language models" --year-from 2020 --year-to 2024

# Sort by citations (most cited first)
python verifisci.py search "reinforcement learning" --sort citations

# Search arXiv preprints only
python verifisci.py search "diffusion models" --source arxiv

# Human-readable output (instead of JSON)
python verifisci.py search "transformer architecture" --text

# BibTeX output for all results
python verifisci.py search "graph neural networks" --bibtex
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
python verifisci.py cite 10.1038/nature14539

# APA format
python verifisci.py cite 10.1038/nature14539 --format apa

# MLA format
python verifisci.py cite 10.1038/nature14539 --format mla

# From arXiv ID
python verifisci.py cite 1706.03762 --type ARXIV

# From URL
python verifisci.py cite "https://arxiv.org/abs/1706.03762" --type URL
```

### `get` — Get full paper details

```bash
# JSON output (default)
python verifisci.py get 10.1038/nature14539

# Human-readable
python verifisci.py get 10.1038/nature14539 --text

# BibTeX for a specific paper
python verifisci.py get 10.1038/nature14539 --bibtex
```

### `read` — Read paper content (abstract, conclusions, full text)

This is the critical command for LLM agents writing articles: it extracts what the paper actually says so the agent can cite it accurately without hallucinating.

```bash
# Read an arXiv paper (gets metadata + abstract + full text + conclusions)
python verifisci.py read https://arxiv.org/abs/1706.03762 --type URL

# Read by arXiv ID
python verifisci.py read 1706.03762 --type ARXIV

# Read a paper by DOI (gets metadata + abstract)
python verifisci.py read 10.1038/nature14539

# Control how much text to extract
python verifisci.py read 1706.03762 --type ARXIV --max-chars 10000

# Human-readable output (shows abstract, conclusions, and full text sections)
python verifisci.py read 1706.03762 --type ARXIV --text
```

**What `read` extracts:**
- **Metadata**: title, authors, year, DOI, URL
- **Abstract**: full paper abstract
- **Conclusions**: the conclusion/discussion section automatically extracted from the PDF
- **Full text**: the first ~20,000 characters of the paper (for arXiv/open-access papers)

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

1. **Find sources:** `python verifisci.py search "topic" --limit 10 --json`
2. **Read the paper:** `python verifisci.py read ARXIV_ID_OR_DOI --text` — extracts abstract, conclusions, and full text so the agent knows what the paper actually says
3. **Get details:** `python verifisci.py get DOI --json`
4. **Generate citation:** `python verifisci.py cite DOI --format bibtex`
5. **Insert into document:** Use the BibTeX key and citation in your LaTeX/Markdown

Example agent workflow:
```bash
# Agent searches for references
python verifisci.py search "transformer attention mechanism" --limit 5 --json

# Agent reads the most relevant paper to understand it
python verifisci.py read 1706.03762 --type ARXIV --text

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
