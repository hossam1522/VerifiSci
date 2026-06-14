#!/usr/bin/env python3
"""
VerifiSci - CLI tool for searching academic sources and generating citations.
Designed for LLM agents to find reliable academic references for writing articles/theses.

Sources: Semantic Scholar, Google Scholar, CrossRef, OpenAlex, arXiv

Usage:
  python verifisci.py search "transformer attention mechanism" --source all --limit 10 --year-from 2020
  python verifisci.py search "deep learning" --source semantic --limit 5 --json
  python verifisci.py cite 10.1038/nature14539
  python verifisci.py read 1706.03762 --type ARXIV
"""

import argparse
import json
import sys
import time
import os
import hashlib
import textwrap
from typing import Optional
from urllib.parse import quote_plus

import requests

# ---------------------------------------------------------------------------
# Cache utilities
# ---------------------------------------------------------------------------

CACHE_DIR = os.path.expanduser("~/.cache/verifisci")
os.makedirs(CACHE_DIR, exist_ok=True)


def _cache_key(prefix: str, *args) -> str:
    raw = prefix + "|".join(str(a) for a in args)
    return hashlib.md5(raw.encode()).hexdigest()


def _cache_get(key: str, max_age_s: int = 3600):
    path = os.path.join(CACHE_DIR, key)
    if os.path.exists(path):
        if time.time() - os.path.getmtime(path) < max_age_s:
            with open(path) as f:
                return json.load(f)
    return None


def _cache_set(key: str, data):
    with open(os.path.join(CACHE_DIR, key), "w") as f:
        json.dump(data, f)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

SESSION = requests.Session()
SESSION.headers.update({
    "User-Agent": "VerifiSci/1.0 (https://github.com/anomalyco/verifisci; mailto:verifisci@example.com)"
})
SESSION.headers.update({"Accept": "application/json"})

def safe_get(url: str, params: dict = None, timeout: int = 15, max_retries: int = 3):
    """GET with retries and exponential backoff."""
    last_exc = None
    for attempt in range(max_retries):
        try:
            r = SESSION.get(url, params=params, timeout=timeout)
            if r.status_code == 429:
                last_exc = requests.HTTPError(
                    f"429 Too Many Requests from {url}", response=r)
                wait = 2 ** attempt
                time.sleep(wait)
                continue
            r.raise_for_status()
            return r
        except requests.RequestException as e:
            last_exc = e
            if attempt == max_retries - 1:
                raise
            # Only retry on transient errors
            if getattr(e, "response", None) is not None and e.response.status_code == 429:
                time.sleep(2 ** attempt)
                continue
            time.sleep(1 * (attempt + 1))
    raise last_exc or Exception(f"Max retries exceeded for {url}")


def make_bibtex_key(authors: list, year: str, title: str) -> str:
    """Generate a BibTeX citation key from author, year, title."""
    last = "unknown"
    if authors:
        parts = authors[0].split()
        if parts:
            last = parts[-1]
    y = year or "????"
    first_word = "x"
    if title and title.strip():
        words = title.strip().split()
        if words:
            first_word = words[0].lower().rstrip(".,;:()[]{}?!\"'")
    return f"{last}{y}{first_word}"


def format_bibtex(paper: dict) -> str:
    """Generate BibTeX entry from paper dict."""
    cite_key = paper.get("bibtex_key", "")
    if not cite_key:
        last = (paper.get("authors", [""])[0].split()[-1] if paper.get("authors") else "unknown")
        year = paper.get("year", "????")
        title_first = (paper.get("title", "untitled") or "untitled").split()[0].lower().rstrip(".,;:")
        cite_key = f"{last}{year}{title_first}"

    etype = paper.get("type", "article")
    bib = [f"@{etype}{{{cite_key},"]

    fields = {
        "title": paper.get("title", ""),
        "author": " and ".join(paper.get("authors", [])),
        "journal": paper.get("journal", paper.get("venue", "")),
        "year": paper.get("year", ""),
        "volume": paper.get("volume", ""),
        "number": paper.get("issue", ""),
        "pages": paper.get("pages", ""),
        "publisher": paper.get("publisher", ""),
        "doi": paper.get("doi", ""),
        "url": paper.get("url", ""),
    }

    for k, v in fields.items():
        if v:
            bib.append(f"  {k} = {{{v}}},")

    bib.append("}")
    return "\n".join(bib)


def paper_dict(title="", authors=None, year="", doi="", url="", venue="",
               citations=0, abstract="", source="", pdf_url="", bibtex_key="",
               type_="article", journal="", volume="", issue="", pages="",
               publisher="", arxiv_id="") -> dict:
    return {
        "title": title,
        "authors": authors or [],
        "year": str(year),
        "doi": doi,
        "url": url,
        "venue": venue,
        "citations": citations,
        "abstract": abstract,
        "source": source,
        "pdf_url": pdf_url,
        "bibtex_key": bibtex_key,
        "type": type_,
        "journal": journal,
        "volume": volume,
        "issue": issue,
        "pages": pages,
        "publisher": publisher,
        "arxiv_id": arxiv_id,
    }


# ===================================================================
# Semantic Scholar API (free, no key required)
# ===================================================================

SEMANTIC_BASE = "https://api.semanticscholar.org/graph/v1"

# Semantic Scholar API key (set via env var or parameter)
SEMANTIC_API_KEY = os.environ.get("SEMANTIC_SCHOLAR_API_KEY", "")

def set_semantic_api_key(key: str):
    global SEMANTIC_API_KEY
    SEMANTIC_API_KEY = key


def search_semantic_scholar(query: str, limit: int = 10, year_from: str = "",
                            year_to: str = "", sort: str = "relevance") -> list:
    """Search Semantic Scholar.
    sort: 'relevance', 'citationCount:desc', 'publicationDate:desc'
    Requires API key for reliable access (set SEMANTIC_SCHOLAR_API_KEY env var).
    """
    if not SEMANTIC_API_KEY:
        return []  # Skip silently without API key

    params = {
        "query": query,
        "limit": limit,
        "fields": "title,authors,year,abstract,url,venue,externalIds,citationCount,"
                  "publicationDate,openAccessPdf,journal,volume,pages",
    }
    if year_from:
        params["year"] = f"{year_from}-"
    if year_to and not year_from:
        params["year"] = f"-{year_to}"
    elif year_to and year_from:
        params["year"] = f"{year_from}-{year_to}"
    if sort != "relevance":
        params["sort"] = sort

    cache_k = _cache_key("semantic", query, limit, year_from, year_to, sort, SEMANTIC_API_KEY)
    cached = _cache_get(cache_k, max_age_s=1800)
    if cached:
        return cached

    try:
        headers = {}
        if SEMANTIC_API_KEY:
            headers["x-api-key"] = SEMANTIC_API_KEY
        r = safe_get(f"{SEMANTIC_BASE}/paper/search", params=params)
        data = r.json()
    except Exception as e:
        print(f"[WARN] Semantic Scholar failed: {e}", file=sys.stderr)
        return []

    results = []
    for p in data.get("data", []):
        authors = [a["name"] for a in p.get("authors", [])]
        eids = p.get("externalIds", {}) or {}
        doi = eids.get("DOI", "")
        arxiv_id = eids.get("ArXiv", "")
        pdf = p.get("openAccessPdf", {}) or {}
        journal = p.get("journal") or {}
        jname = ""
        if journal:
            jname = journal.get("name", "")

        results.append(paper_dict(
            title=p.get("title", ""),
            authors=authors,
            year=p.get("year", ""),
            doi=doi,
            url=f"https://doi.org/{doi}" if doi else p.get("url", ""),
            venue=p.get("venue", ""),
            journal=jname,
            citations=p.get("citationCount", 0),
            abstract=p.get("abstract", ""),
            source="semantic_scholar",
            pdf_url=pdf.get("url", ""),
            volume=journal.get("volume", "") if journal else "",
            pages=journal.get("pages", "") if journal else "",
            bibtex_key=make_bibtex_key(authors, p.get("year", ""), p.get("title", "")),
            arxiv_id=arxiv_id,
        ))

    _cache_set(cache_k, results)
    return results


def get_semantic_paper(identifier: str, id_type: str = "DOI") -> Optional[dict]:
    """Get paper details from Semantic Scholar by DOI, ArXiv ID, etc."""
    cache_k = _cache_key("semantic_paper", identifier, id_type)
    cached = _cache_get(cache_k, max_age_s=3600)
    if cached:
        return cached

    try:
        r = safe_get(f"{SEMANTIC_BASE}/paper/{id_type}:{identifier}",
                     params={"fields": "title,authors,year,abstract,url,venue,externalIds,"
                                      "citationCount,publicationDate,openAccessPdf,journal,"
                                      "volume,pages,referenceCount,tldr"})
        p = r.json()
    except Exception:
        return None

    authors = [a["name"] for a in p.get("authors", [])]
    eids = p.get("externalIds", {}) or {}
    doi = eids.get("DOI", identifier if id_type == "DOI" else "")
    pdf = p.get("openAccessPdf", {}) or {}
    tldr = p.get("tldr", {}) or {}
    journal = p.get("journal") or {}
    abstract = p.get("abstract", "")
    if tldr.get("text") and not abstract:
        abstract = tldr.get("text", "")
    result = paper_dict(
        title=p.get("title", ""),
        authors=authors,
        year=p.get("year", ""),
        doi=doi,
        url=f"https://doi.org/{doi}" if doi else p.get("url", ""),
        venue=p.get("venue", ""),
        journal=journal.get("name", ""),
        citations=p.get("citationCount", 0),
        abstract=abstract,
        source="semantic_scholar",
        pdf_url=pdf.get("url", ""),
        volume=journal.get("volume", "") if journal else "",
        pages=journal.get("pages", "") if journal else "",
        bibtex_key=make_bibtex_key(authors, p.get("year", ""), p.get("title", "")),
    )
    _cache_set(cache_k, result)
    return result


# ===================================================================
# Google Scholar (via scholarly)
# ===================================================================

def search_google_scholar(query: str, limit: int = 10, year_from: str = "",
                          year_to: str = "", use_proxy: bool = False) -> list:
    """Search Google Scholar using scholarly library.
    WARNING: Google Scholar may rate-limit or block IPs. Use a proxy for heavy usage.
    """
    cache_k = _cache_key("googlescholar", query, limit, year_from, year_to)
    cached = _cache_get(cache_k, max_age_s=3600)
    if cached:
        return cached

    try:
        import scholarly
    except ImportError:
        print("[ERROR] Install scholarly: pip install scholarly", file=sys.stderr)
        return []

    try:
        # Use proxy if requested
        if use_proxy:
            from scholarly import ProxyGenerator
            pg = ProxyGenerator()
            success = pg.FreeProxies()
            if success:
                scholarly.scholarly.use_proxy(pg)
            else:
                print("[WARN] Failed to set up free proxy for Google Scholar", file=sys.stderr)

        search_query = scholarly.search_pubs(query)
        results = []
        count = 0

        for pub in search_query:
            if count >= limit:
                break

            bib = pub.get("bib", {})
            year_str = bib.get("pub_year", "")

            # Year filter
            if year_from and str(year_str) < year_from:
                continue
            if year_to and str(year_str) > year_to:
                continue

            authors = bib.get("author", [])
            if isinstance(authors, str):
                authors = [a.strip() for a in authors.split("and")]

            title = bib.get("title", "")

            results.append(paper_dict(
                title=title,
                authors=authors,
                year=year_str,
                doi="",
                url=bib.get("pub_url", pub.get("pub_url", pub.get("url_scholarbib", ""))),
                venue=bib.get("venue", bib.get("journal", "")),
                journal=bib.get("journal", ""),
                citations=bib.get("num_citations", 0),
                abstract=bib.get("abstract", ""),
                source="google_scholar",
                volume=bib.get("volume", ""),
                pages=bib.get("pages", ""),
                publisher=bib.get("publisher", ""),
                bibtex_key=make_bibtex_key(authors, year_str, title),
            ))
            count += 1

        _cache_set(cache_k, results)
        return results

    except Exception as e:
        print(f"[WARN] Google Scholar failed: {e}", file=sys.stderr)
        return []


# ===================================================================
# CrossRef API (free, no key, more polite - use a User-Agent)
# ===================================================================

CROSSREF_BASE = "https://api.crossref.org/works"

def search_crossref(query: str, limit: int = 10, year_from: str = "",
                    year_to: str = "", sort: str = "relevance") -> list:
    """Search CrossRef for DOI-registered works."""
    cache_k = _cache_key("crossref", query, limit, year_from, year_to, sort)
    cached = _cache_get(cache_k, max_age_s=3600)
    if cached:
        return cached

    params = {
        "query": query,
        "rows": limit,
        "sort": sort,
    }
    if year_from:
        params["filter"] = f"from-pub-date:{year_from}-01-01"
    if year_to:
        flt = params.get("filter", "")
        flt += f",until-pub-date:{year_to}-12-31" if flt else f"until-pub-date:{year_to}-12-31"
        params["filter"] = flt

    try:
        r = safe_get(CROSSREF_BASE, params=params)
        data = r.json()
    except Exception as e:
        print(f"[WARN] CrossRef failed: {e}", file=sys.stderr)
        return []

    results = []
    for item in data.get("message", {}).get("items", []):
        authors = []
        for a in item.get("author", []):
            fam = a.get("family", "")
            giv = a.get("given", "")
            authors.append(f"{giv} {fam}".strip() or fam)

        doi = item.get("DOI", "")
        pub_date = item.get("published-print", {}) or item.get("published-online", {}) or {}
        date_parts = pub_date.get("date-parts", [[]])
        year = str(date_parts[0][0]) if date_parts and date_parts[0] else ""

        title_list = item.get("title", [])
        title = title_list[0] if title_list else ""

        container = item.get("container-title", [])
        journal = container[0] if container else ""
        venue = item.get("publisher", "")

        is_referenced = item.get("is-referenced-by-count", 0)

        results.append(paper_dict(
            title=title,
            authors=authors,
            year=year,
            doi=doi,
            url=f"https://doi.org/{doi}" if doi else "",
            venue=venue,
            journal=journal,
            citations=is_referenced,
            abstract=item.get("abstract", ""),
            source="crossref",
            volume=item.get("volume", ""),
            issue=item.get("issue", ""),
            pages=item.get("page", ""),
            publisher=item.get("publisher", ""),
            bibtex_key=make_bibtex_key(authors, year, title),
        ))

    _cache_set(cache_k, results)
    return results


# ===================================================================
# OpenAlex API (free, no key, very comprehensive)
# ===================================================================

OPENALEX_BASE = "https://api.openalex.org/works"

def search_openalex(query: str, limit: int = 10, year_from: str = "",
                    year_to: str = "", sort: str = "relevance") -> list:
    """Search OpenAlex for academic works."""
    cache_k = _cache_key("openalex", query, limit, year_from, year_to, sort)
    cached = _cache_get(cache_k, max_age_s=3600)
    if cached:
        return cached

    # Map sort values to OpenAlex format
    sort_map = {
        "relevance": "relevance_score:desc",
        "citations": "cited_by_count:desc",
        "date": "publication_date:desc",
    }
    oa_sort = sort_map.get(sort, "relevance_score:desc")

    params = {
        "search": query,
        "per_page": min(limit, 200),
        "sort": oa_sort,
    }
    filter_parts = []
    if year_from:
        filter_parts.append(f"publication_year:>{int(year_from)-1}")
    if year_to:
        filter_parts.append(f"publication_year:<{int(year_to)+1}")
    if filter_parts:
        params["filter"] = ",".join(filter_parts)

    try:
        r = safe_get(OPENALEX_BASE, params=params)
        data = r.json()
    except Exception as e:
        print(f"[WARN] OpenAlex failed: {e}", file=sys.stderr)
        return []

    results = []
    for item in data.get("results", []):
        authorship = item.get("authorships", [])
        authors = [a.get("author", {}).get("display_name", "") for a in authorship]

        doi = (item.get("doi") or "").replace("https://doi.org/", "")
        primary_loc = item.get("primary_location", {}) or {}
        source_info = primary_loc.get("source", {}) or {}

        # Reconstruct abstract from inverted index
        abstract = ""
        inv_idx = item.get("abstract_inverted_index") or {}
        if inv_idx:
            words = sorted(inv_idx.items(), key=lambda x: x[1][0] if x[1] else 0)
            abstract = " ".join(w[0] for w in words)

        year_str = str(item.get("publication_year", ""))
        title = item.get("title", "") or ""

        results.append(paper_dict(
            title=title,
            authors=authors,
            year=year_str,
            doi=doi if doi else (item.get("ids", {}).get("doi", "")),
            url=item.get("doi", "") or f"https://openalex.org/{item.get('id','')}",
            venue=source_info.get("display_name", ""),
            journal=source_info.get("display_name", ""),
            citations=item.get("cited_by_count", 0),
            abstract=abstract,
            source="openalex",
            pdf_url=(primary_loc.get("pdf_url", "") if primary_loc else ""),
            volume=item.get("biblio", {}).get("volume", ""),
            issue=item.get("biblio", {}).get("issue", ""),
            pages=f"{item.get('biblio',{}).get('first_page','')}-{item.get('biblio',{}).get('last_page','')}".strip("-"),
            publisher=(item.get("host_venue") or {}).get("publisher") or ((item.get("primary_location") or {}).get("source") or {}).get("host_organization_name", ""),
            bibtex_key=make_bibtex_key(authors, year_str, title),
        ))

    _cache_set(cache_k, results)
    return results


# ===================================================================
# arXiv API
# ===================================================================

ARXIV_BASE = "http://export.arxiv.org/api/query"

def search_arxiv(query: str, limit: int = 10) -> list:
    """Search arXiv for preprints."""
    cache_k = _cache_key("arxiv", query, limit)
    cached = _cache_get(cache_k, max_age_s=3600)
    if cached:
        return cached

    params = {
        "search_query": f"all:{query}",
        "start": 0,
        "max_results": limit,
        "sortBy": "relevance",
    }

    try:
        r = safe_get(ARXIV_BASE, params=params, timeout=20)
        import xml.etree.ElementTree as ET
        root = ET.fromstring(r.text)
    except Exception as e:
        print(f"[WARN] arXiv failed: {e}", file=sys.stderr)
        return []

    atom_ns = "http://www.w3.org/2005/Atom"
    arxiv_ns = "http://arxiv.org/schemas/atom"

    results = []
    for entry in root.findall(f"{{{atom_ns}}}entry"):
        title_el = entry.find(f"{{{atom_ns}}}title")
        title = title_el.text.strip().replace("\n", " ") if title_el is not None and title_el.text else ""

        id_el = entry.find(f"{{{atom_ns}}}id")
        arxiv_id_raw = id_el.text.strip() if id_el is not None and id_el.text else ""
        arxiv_id = arxiv_id_raw.replace("http://arxiv.org/abs/", "") if arxiv_id_raw else ""
        pdf_url = arxiv_id_raw.replace("/abs/", "/pdf/") if arxiv_id_raw else ""

        authors = []
        for a in entry.findall(f"{{{atom_ns}}}author"):
            name_el = a.find(f"{{{atom_ns}}}name")
            if name_el is not None and name_el.text:
                authors.append(name_el.text)

        summary_el = entry.find(f"{{{atom_ns}}}summary")
        abstract = summary_el.text.strip().replace("\n", " ") if summary_el is not None and summary_el.text else ""

        published_el = entry.find(f"{{{atom_ns}}}published")
        published = published_el.text if published_el is not None and published_el.text else ""
        year = published[:4] if published else ""

        results.append(paper_dict(
            title=title,
            authors=authors,
            year=year,
            doi="",
            url=arxiv_id_raw,
            venue="arXiv",
            citations=0,
            abstract=abstract,
            source="arxiv",
            pdf_url=pdf_url,
            bibtex_key=make_bibtex_key(authors, year, title),
            arxiv_id=arxiv_id,
            type_="article",
        ))

    _cache_set(cache_k, results)
    return results


# ===================================================================
# Combined search
# ===================================================================

SOURCES = {
    "semantic": search_semantic_scholar,
    "google": search_google_scholar,
    "crossref": search_crossref,
    "openalex": search_openalex,
    "arxiv": search_arxiv,
}


def search_all(query: str, limit: int = 10, year_from: str = "",
               year_to: str = "", sort: str = "relevance",
               sources: list = None) -> dict:
    """Search across multiple sources and de-duplicate by DOI/title."""
    if sources is None:
        sources = ["semantic", "crossref", "openalex"]

    # Map sort values for each source
    source_sort_map = {
        "semantic": {
            "relevance": "relevance",
            "citations": "citationCount:desc",
            "date": "publicationDate:desc",
        },
        "crossref": {
            "relevance": "relevance",
            "citations": "is-referenced-by-count",
            "date": "published",
        },
        "openalex": {
            "relevance": "relevance_score:desc",
            "citations": "cited_by_count:desc",
            "date": "publication_date:desc",
        },
    }

    # Search each source
    source_results = {}
    for src in sources:
        fn = SOURCES.get(src)
        if not fn:
            continue
        src_limit = max(5, limit)
        extra = {"query": query, "limit": src_limit}
        if src == "arxiv":
            papers = fn(query, src_limit)
        else:
            extra["year_from"] = year_from
            extra["year_to"] = year_to
            if src in source_sort_map:
                extra["sort"] = source_sort_map[src].get(sort, "relevance")
            papers = fn(**extra)
        source_results[src] = papers

    # Interleave results from different sources, then deduplicate
    seen = set()
    all_papers = []
    max_len = max((len(v) for v in source_results.values()), default=0)
    for i in range(max_len):
        for src in sources:
            papers = source_results.get(src, [])
            if i < len(papers):
                p = papers[i]
                key = p["doi"].lower() if p["doi"] else p["title"].lower().strip()[:100]
                if key and key not in seen:
                    seen.add(key)
                    all_papers.append(p)

    # Sort by citations if requested (within the interleaved/de-duped set)
    if sort == "citations":
        all_papers.sort(key=lambda p: p["citations"], reverse=True)
    elif sort == "date":
        all_papers.sort(key=lambda p: p["year"], reverse=True)

    return {
        "query": query,
        "total_results": len(all_papers),
        "results": all_papers[:limit],
        "sources_used": sources,
    }


# ===================================================================
# Citation generation
# ===================================================================

def generate_citation(identifier: str, id_type: str = "DOI",
                      format_: str = "bibtex") -> Optional[str]:
    """Look up a paper and generate citation in specified format."""
    paper = None

    # Try Semantic Scholar first
    if id_type.upper() in ("DOI", "ARXIV", "PMID", "CORPUSID"):
        paper = get_semantic_paper(identifier, id_type)
    elif id_type.upper() == "URL":
        # Extract DOI from URL if possible
        if "doi.org/" in identifier:
            doi = identifier.split("doi.org/")[-1].strip()
            paper = get_semantic_paper(doi, "DOI")
        elif "arxiv.org/abs/" in identifier:
            arxiv_id = identifier.split("arxiv.org/abs/")[-1].strip().rstrip("/")
            paper = get_semantic_paper(arxiv_id, "ARXIV")

    # Fallback: OpenAlex
    if not paper:
        oa_id = identifier
        if id_type.upper() == "ARXIV":
            oa_id = f"https://arxiv.org/abs/{identifier}"
        elif id_type.upper() == "DOI" and not oa_id.startswith("http"):
            oa_id = f"https://doi.org/{identifier}"
        try:
            r = safe_get(f"https://api.openalex.org/works/{oa_id}")
            item = r.json()
            if item and item.get("title"):
                authorship = item.get("authorships", [])
                authors = [a.get("author", {}).get("display_name", "") for a in authorship]
                doi = (item.get("doi") or "").replace("https://doi.org/", "")
                title = item.get("title", "") or ""
                year = str(item.get("publication_year", ""))
                primary_loc = item.get("primary_location", {}) or {}
                source_info = primary_loc.get("source", {}) or {}
                inv_idx = item.get("abstract_inverted_index") or {}
                abstract = ""
                if inv_idx:
                    words = sorted(inv_idx.items(), key=lambda x: x[1][0] if x[1] else 0)
                    abstract = " ".join(w[0] for w in words)
                paper = paper_dict(
                    title=title, authors=authors, year=year, doi=doi,
                    url=item.get("doi", f"https://doi.org/{doi}") if doi else "",
                    venue=source_info.get("display_name", ""),
                    journal=source_info.get("display_name", ""),
                    citations=item.get("cited_by_count", 0), abstract=abstract,
                    source="openalex",
                    pdf_url=(primary_loc.get("pdf_url", "") if primary_loc else ""),
                    volume=item.get("biblio", {}).get("volume", ""),
                    issue=item.get("biblio", {}).get("issue", ""),
                    pages=f"{item.get('biblio',{}).get('first_page','')}-{item.get('biblio',{}).get('last_page','')}".strip("-"),
                    publisher=item.get("host_venue", {}).get("publisher", ""),
                    bibtex_key=make_bibtex_key(authors, year, title),
                )
        except Exception as e:
            pass

    # Fallback: CrossRef
    if not paper and id_type.upper() == "DOI":
        try:
            r = safe_get(f"{CROSSREF_BASE}/{identifier}")
            item = r.json().get("message", {})
            if item:
                authors = []
                for a in item.get("author", []):
                    fam = a.get("family", "")
                    giv = a.get("given", "")
                    authors.append(f"{giv} {fam}".strip() or fam)
                pub_date = item.get("published-print", {}) or item.get("published-online", {}) or {}
                dp = pub_date.get("date-parts", [[]])
                year = str(dp[0][0]) if dp and dp[0] else ""
                title = (item.get("title", [""]) or [""])[0]
                paper = paper_dict(
                    title=title, authors=authors, year=year, doi=identifier,
                    url=f"https://doi.org/{identifier}",
                    venue=item.get("publisher", ""),
                    journal=(item.get("container-title", [""]) or [""])[0],
                    volume=item.get("volume", ""), issue=item.get("issue", ""),
                    pages=item.get("page", ""), publisher=item.get("publisher", ""),
                    source="crossref",
                    bibtex_key=make_bibtex_key(authors, year, title),
                )
        except Exception as e:
            print(f"[WARN] CrossRef lookup failed: {e}", file=sys.stderr)

    if not paper:
        return None

    if format_ == "bibtex":
        return format_bibtex(paper)
    elif format_ == "apa":
        authors = ", ".join(paper.get("authors", []))
        year = paper.get("year", "(n.d.)")
        title = paper.get("title", "")
        journal = paper.get("journal", paper.get("venue", ""))
        volume = paper.get("volume", "")
        pages = paper.get("pages", "")
        doi = paper.get("doi", "")
        apa = f"{authors} ({year}). {title}."
        if journal:
            apa += f" {journal}"
            if volume:
                apa += f", {volume}"
                if pages:
                    apa += f", {pages}"
            apa += "."
        if doi:
            apa += f" https://doi.org/{doi}"
        return apa
    elif format_ == "mla":
        authors = paper.get("authors", [])
        first_author = authors[0] if authors else ""
        title = paper.get("title", "")
        journal = paper.get("journal", paper.get("venue", ""))
        year = paper.get("year", "")
        mla = f'{first_author}. "{title}." {journal} ({year}).'
        doi = paper.get("doi", "")
        if doi:
            mla += f" doi:{doi}."
        return mla

    return format_bibtex(paper)


# ===================================================================
# Read paper content (abstract, conclusions, full text for arXiv)
# ===================================================================

def extract_section(text: str, section_names: list, max_chars: int = 4000) -> str:
    """Extract text under common section headings like Conclusion, Abstract, etc."""
    lines = text.split("\n")
    in_section = False
    collected = []
    chars = 0

    for line in lines:
        stripped = line.strip().lower().rstrip(".")
        # Check if we're entering a target section
        if any(stripped == s.lower() or stripped.startswith(s.lower())
               for s in section_names):
            in_section = True
            continue
        # Check if we're entering a new section (exit current)
        if in_section and (
            stripped in ["abstract", "introduction", "related work", "references",
                         "acknowledgments", "appendix", "method", "methods",
                         "methodology", "results", "discussion", "conclusion",
                         "background", "preliminaries", "experiments", "evaluation",
                         "limitations", "future work", "bibliography"]
            or (stripped and (
                stripped[0].isdigit() and "." in stripped[:4] and len(stripped) < 60
            ))
        ):
            if stripped not in ["conclusion", "conclusions", "concluding remarks",
                                "summary", "discussion and conclusion"]:
                break

        if in_section and stripped:
            collected.append(line.strip())
            chars += len(line)
            if chars > max_chars:
                collected.append("[...truncated...]")
                break

    return " ".join(collected) if collected else ""


def read_paper(identifier: str, id_type: str = "DOI",
               max_content_chars: int = 20000) -> dict:
    """Read a paper: get metadata, abstract, and attempt to get full text.
    Returns a dict with: metadata, abstract, full_text (if available),
    conclusions, key_points.
    """
    result = {
        "identifier": identifier,
        "id_type": id_type,
        "metadata": None,
        "abstract": "",
        "full_text": "",
        "conclusions": "",
        "source_of_text": "none",
        "error": None,
    }

    paper = None
    arxiv_id = None

    # Determine arXiv ID if applicable
    if id_type.upper() == "ARXIV":
        arxiv_id = identifier
    elif "arxiv.org/abs/" in identifier:
        arxiv_id = identifier.split("arxiv.org/abs/")[-1].strip().rstrip("/").rstrip(".pdf")
    elif "arxiv.org/pdf/" in identifier:
        arxiv_id = identifier.split("arxiv.org/pdf/")[-1].strip().rstrip("/").rstrip(".pdf")

    # Step 1a: For arXiv papers, get metadata from arXiv API
    if arxiv_id:
        try:
            import xml.etree.ElementTree as ET
            r = safe_get(f"http://export.arxiv.org/api/query?id_list={arxiv_id}&max_results=1", timeout=15)
            atom_ns = "http://www.w3.org/2005/Atom"
            root = ET.fromstring(r.text)
            entry = root.find(f"{{{atom_ns}}}entry")
            if entry is not None:
                title_el = entry.find(f"{{{atom_ns}}}title")
                title = title_el.text.strip().replace("\n", " ") if title_el is not None and title_el.text else ""
                authors = []
                for a in entry.findall(f"{{{atom_ns}}}author"):
                    name_el = a.find(f"{{{atom_ns}}}name")
                    if name_el is not None and name_el.text:
                        authors.append(name_el.text)
                summary_el = entry.find(f"{{{atom_ns}}}summary")
                abstract = summary_el.text.strip().replace("\n", " ") if summary_el is not None and summary_el.text else ""
                published_el = entry.find(f"{{{atom_ns}}}published")
                published = published_el.text if published_el is not None and published_el.text else ""
                year = published[:4] if published else ""
                paper = paper_dict(
                    title=title, authors=authors, year=year,
                    url=f"http://arxiv.org/abs/{arxiv_id}",
                    venue="arXiv", abstract=abstract, source="arxiv",
                    pdf_url=f"https://arxiv.org/pdf/{arxiv_id}.pdf",
                    bibtex_key=make_bibtex_key(authors, year, title),
                    arxiv_id=arxiv_id,
                )
        except Exception as e:
            result["error"] = f"arXiv metadata failed: {e}"

    # Step 1b: For non-arxiv papers, get metadata from OpenAlex
    if not paper and not arxiv_id:
        oa_id = identifier
        if id_type.upper() == "DOI" and not identifier.startswith("http"):
            oa_id = f"https://doi.org/{identifier}"
        try:
            r = safe_get(f"https://api.openalex.org/works/{oa_id}")
            item = r.json()
            if item and item.get("title"):
                authorship = item.get("authorships", [])
                authors = [a.get("author", {}).get("display_name", "") for a in authorship]
                doi = (item.get("doi") or "").replace("https://doi.org/", "")
                title = item.get("title", "") or ""
                year = str(item.get("publication_year", ""))
                primary_loc = item.get("primary_location", {}) or {}
                source_info = primary_loc.get("source", {}) or {}

                inv_idx = item.get("abstract_inverted_index") or {}
                abstract = ""
                if inv_idx:
                    words = sorted(inv_idx.items(), key=lambda x: x[1][0] if x[1] else 0)
                    abstract = " ".join(w[0] for w in words)

                paper = paper_dict(
                    title=title, authors=authors, year=year, doi=doi,
                    url=item.get("doi", f"https://doi.org/{doi}") if doi else "",
                    venue=source_info.get("display_name", ""),
                    journal=source_info.get("display_name", ""),
                    citations=item.get("cited_by_count", 0), abstract=abstract,
                    source="openalex",
                    pdf_url=(primary_loc.get("pdf_url", "") if primary_loc else ""),
                    volume=item.get("biblio", {}).get("volume", ""),
                    issue=item.get("biblio", {}).get("issue", ""),
                    pages=f"{item.get('biblio',{}).get('first_page','')}-{item.get('biblio',{}).get('last_page','')}".strip("-"),
                    publisher=(item.get("host_venue") or {}).get("publisher", ""),
                    bibtex_key=make_bibtex_key(authors, year, title),
                    arxiv_id="",
                )
        except Exception as e:
            result["error"] = f"OpenAlex lookup failed: {e}"

    result["metadata"] = paper
    result["abstract"] = paper["abstract"] if paper else ""

    # Step 2: Try to get full text from PDF
    pdf_url = paper.get("pdf_url", "") if paper else ""

    # For arXiv papers, PDF URL is always available
    if arxiv_id:
        pdf_url = f"https://arxiv.org/pdf/{arxiv_id}.pdf"
        try:
            text = download_and_extract_pdf_text(pdf_url, max_chars=max_content_chars)
            if text:
                result["source_of_text"] = "pdf"
                result["full_text"] = text[:max_content_chars]

                # Extract conclusions
                conclusion_sections = [
                    "conclusion", "conclusions", "concluding remarks",
                    "discussion and conclusion", "summary and conclusion",
                    "summary", "final remarks", "discussion",
                ]
                result["conclusions"] = extract_section(text, conclusion_sections, 5000)

                # If no conclusion found, take last ~3000 chars
                if not result["conclusions"] and len(text) > 3000:
                    result["conclusions"] = text[-3000:]
        except Exception as e:
            result["error"] = (result.get("error") or "") + f"PDF extraction failed: {e}"

    return result


def download_and_extract_pdf_text(url: str, max_chars: int = 20000) -> str:
    """Download a PDF and extract its text content."""
    try:
        r = safe_get(url, timeout=30)
        if r.status_code != 200:
            return ""

        # Save to temp file and extract with PyPDF2
        import tempfile
        import io

        try:
            from PyPDF2 import PdfReader
        except ImportError:
            # Fallback: try pdftotext command line tool
            import subprocess
            import tempfile
            with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
                f.write(r.content)
                tmp_path = f.name
            try:
                result = subprocess.run(
                    ["pdftotext", "-l", "20", tmp_path, "-"],
                    capture_output=True, text=True, timeout=30
                )
                return result.stdout[:max_chars]
            finally:
                os.unlink(tmp_path)

        pdf_file = io.BytesIO(r.content)
        reader = PdfReader(pdf_file)
        text_parts = []
        total_chars = 0
        max_pages = min(len(reader.pages), 30)

        for i in range(max_pages):
            page_text = reader.pages[i].extract_text() or ""
            text_parts.append(page_text)
            total_chars += len(page_text)
            if total_chars > max_chars:
                break

        return "\n\n".join(text_parts)[:max_chars]

    except Exception:
        return ""


# ===================================================================
# Main CLI
# ===================================================================

def main():
    parser = argparse.ArgumentParser(
        description="VerifiSci - Academic paper search and citation tool for LLM agents",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=textwrap.dedent("""\
            Examples:
              # Search papers (default: OpenAlex, free, no key needed)
              %(prog)s search "attention is all you need" --limit 5

              # Search and get JSON output (default format)
              %(prog)s search "graph neural networks" --year-from 2022 --sort citations

              # Search all free sources
              %(prog)s search "reinforcement learning" --source all --limit 20

              # Search arXiv specifically
              %(prog)s search "diffusion models" --source arxiv

              # Read a paper's content (abstract + conclusions + full text)
              %(prog)s read 1706.03762 --type ARXIV

              # Generate BibTeX citation from DOI
              %(prog)s cite 10.1038/nature14539

              # Generate APA citation
              %(prog)s cite 10.1038/nature14539 --format apa

              # Get full paper details
              %(prog)s get 10.1038/nature14539

              # Search with text output (human-readable)
              %(prog)s search "large language models" --limit 3 --text

            Environment variables:
              SEMANTIC_SCHOLAR_API_KEY   API key for Semantic Scholar (higher rate limits)
        """),
    )

    sub = parser.add_subparsers(dest="command", required=True)

    # ---- search command ----
    sp = sub.add_parser("search", help="Search for academic papers")
    sp.add_argument("query", help="Search query string")
    sp.add_argument("--source", "-s", default="openalex",
                    choices=["semantic", "google", "crossref", "openalex", "arxiv",
                             "all"],
                    help="Source to search (default: openalex). 'all' = openalex+crossref+arxiv")
    sp.add_argument("--limit", "-n", type=int, default=10, help="Max results (default: 10)")
    sp.add_argument("--year-from", type=str, default="", help="Filter from year")
    sp.add_argument("--year-to", type=str, default="", help="Filter to year")
    sp.add_argument("--sort", default="relevance",
                    choices=["relevance", "citations", "date"],
                    help="Sort order (default: relevance)")
    sp.add_argument("--json", "-j", action="store_true", help="Output as JSON (default unless --text or --bibtex used)")
    sp.add_argument("--text", "-t", action="store_true", help="Output as readable text")
    sp.add_argument("--bibtex", "-b", action="store_true", help="Output as BibTeX")
    sp.add_argument("--proxy", action="store_true", help="Use proxy for Google Scholar")
    sp.add_argument("--semantic-key", type=str, default="", metavar="KEY",
                    help="Semantic Scholar API key (or set SEMANTIC_SCHOLAR_API_KEY env var)")

    # ---- cite command ----
    cp = sub.add_parser("cite", help="Generate citation for a paper")
    cp.add_argument("identifier", help="DOI, arXiv ID, or URL of the paper")
    cp.add_argument("--type", "-t", default="DOI",
                    choices=["DOI", "ARXIV", "URL", "PMID", "CORPUSID"],
                    help="Identifier type (default: DOI)")
    cp.add_argument("--format", "-f", default="bibtex",
                    choices=["bibtex", "apa", "mla"],
                    help="Citation format (default: bibtex)")

    # ---- get command ----
    gp = sub.add_parser("get", help="Get full details of a paper")
    gp.add_argument("identifier", help="DOI, arXiv ID, or URL")
    gp.add_argument("--type", "-t", default="DOI",
                    choices=["DOI", "ARXIV", "URL", "PMID", "CORPUSID"],
                    help="Identifier type (default: DOI)")
    gp.add_argument("--json", "-j", action="store_true", help="Output as JSON (default)")
    gp.add_argument("--text", action="store_true", help="Output as readable text")
    gp.add_argument("--bibtex", "-b", action="store_true", help="Output as BibTeX")

    # ---- read command ----
    rp = sub.add_parser("read", help="Read paper content: metadata, abstract, conclusions, full text")
    rp.add_argument("identifier", help="DOI, arXiv ID, or URL of the paper")
    rp.add_argument("--type", "-t", default="DOI",
                    choices=["DOI", "ARXIV", "URL"],
                    help="Identifier type (default: DOI)")
    rp.add_argument("--max-chars", type=int, default=20000,
                    help="Max characters of full text (default: 20000)")
    rp.add_argument("--json", "-j", action="store_true", help="Output as JSON (default)")
    rp.add_argument("--text", action="store_true", help="Output as readable text")

    args = parser.parse_args()

    if args.command == "search":
        # Set API key if provided
        if hasattr(args, "semantic_key") and args.semantic_key:
            set_semantic_api_key(args.semantic_key)

        # Resolve sources
        if args.source == "all":
            sources = ["openalex", "crossref", "arxiv"]
        else:
            sources = [args.source]

        result = search_all(
            query=args.query,
            limit=args.limit,
            year_from=args.year_from,
            year_to=args.year_to,
            sort="citations" if args.sort == "citations" else args.sort,
            sources=sources,
        )

        if args.bibtex:
            for p in result["results"]:
                print(format_bibtex(p))
                print()
        elif args.text:
            for i, p in enumerate(result["results"], 1):
                print(f"{i}. {p['title']}")
                print(f"   Authors: {', '.join(p['authors'][:5])}"
                      f"{' et al.' if len(p['authors']) > 5 else ''}")
                print(f"   Year: {p['year']} | Citations: {p['citations']} | Source: {p['source']}")
                if p["doi"]:
                    print(f"   DOI: {p['doi']}")
                if p["url"]:
                    print(f"   URL: {p['url']}")
                if p["pdf_url"]:
                    print(f"   PDF: {p['pdf_url']}")
                if p.get("abstract"):
                    abstract = p["abstract"][:300].replace("\n", " ")
                    print(f"   Abstract: {abstract}...")
                print()
        else:
            # Default: JSON output
            print(json.dumps(result, indent=2, ensure_ascii=False))

    elif args.command == "cite":
        citation = generate_citation(args.identifier, args.type, args.format)
        if citation:
            print(citation)
        else:
            print(f"Could not find paper with {args.type}: {args.identifier}", file=sys.stderr)
            sys.exit(1)

    elif args.command == "get":
        id_type_map = {
            "DOI": "DOI", "ARXIV": "ARXIV", "URL": "URL",
            "PMID": "PMID", "CORPUSID": "CorpusId"
        }
        id_type = id_type_map.get(args.type, "DOI")
        paper = get_semantic_paper(args.identifier, id_type)

        # Fallback to OpenAlex lookup via DOI
        if not paper and (id_type == "DOI" or args.type == "DOI"):
            try:
                r = safe_get(f"https://api.openalex.org/works/https://doi.org/{args.identifier}")
                item = r.json()
                authorship = item.get("authorships", [])
                authors = [a.get("author", {}).get("display_name", "") for a in authorship]
                doi = (item.get("doi") or "").replace("https://doi.org/", "")
                title = item.get("title", "") or ""
                year = str(item.get("publication_year", ""))
                primary_loc = item.get("primary_location", {}) or {}
                source_info = primary_loc.get("source", {}) or {}

                inv_idx = item.get("abstract_inverted_index") or {}
                abstract = ""
                if inv_idx:
                    words = sorted(inv_idx.items(), key=lambda x: x[1][0] if x[1] else 0)
                    abstract = " ".join(w[0] for w in words)

                paper = paper_dict(
                    title=title, authors=authors, year=year, doi=doi,
                    url=item.get("doi", f"https://doi.org/{doi}") if doi else "",
                    venue=source_info.get("display_name", ""),
                    journal=source_info.get("display_name", ""),
                    citations=item.get("cited_by_count", 0), abstract=abstract,
                    source="openalex",
                    pdf_url=(primary_loc.get("pdf_url", "") if primary_loc else ""),
                    volume=item.get("biblio", {}).get("volume", ""),
                    issue=item.get("biblio", {}).get("issue", ""),
                    pages=f"{item.get('biblio',{}).get('first_page','')}-{item.get('biblio',{}).get('last_page','')}".strip("-"),
                    publisher=item.get("host_venue", {}).get("publisher", ""),
                    bibtex_key=make_bibtex_key(authors, year, title),
                )
            except Exception:
                pass

        if paper:
            if args.bibtex:
                print(format_bibtex(paper))
            elif args.text:
                print(f"Title: {paper['title']}")
                print(f"Authors: {', '.join(paper['authors'])}")
                print(f"Year: {paper['year']} | Citations: {paper['citations']}")
                print(f"DOI: {paper['doi']}")
                print(f"URL: {paper['url']}")
                print(f"Journal: {paper.get('journal', '')}")
                if paper.get('abstract'):
                    print(f"Abstract: {paper['abstract'][:500]}...")
            else:
                print(json.dumps(paper, indent=2, ensure_ascii=False))
        else:
            print(f"Could not find paper with {args.type}: {args.identifier}", file=sys.stderr)
            sys.exit(1)

    elif args.command == "read":
        id_type_map = {
            "DOI": "DOI", "ARXIV": "ARXIV", "URL": "URL",
        }
        id_type = id_type_map.get(args.type, "DOI")
        result = read_paper(args.identifier, id_type, max_content_chars=args.max_chars)

        if args.text:
            md = result.get("metadata") or {}
            print(f"{'='*70}")
            print(f"TITLE: {md.get('title', 'N/A')}")
            print(f"AUTHORS: {', '.join(md.get('authors', []))}")
            print(f"YEAR: {md.get('year', 'N/A')} | CITATIONS: {md.get('citations', 0)}")
            print(f"DOI: {md.get('doi', 'N/A')}")
            print(f"URL: {md.get('url', 'N/A')}")
            print(f"{'='*70}")

            if result.get("abstract"):
                print(f"\n--- ABSTRACT ---")
                print(result["abstract"][:2000])

            if result.get("conclusions"):
                print(f"\n--- CONCLUSIONS ---")
                print(result["conclusions"][:3000])

            if result.get("full_text"):
                print(f"\n--- FULL TEXT (source: {result['source_of_text']}) ---")
                print(result["full_text"][:result.get("max_chars", 5000)])

            if result.get("error"):
                print(f"\n[NOTE] {result['error']}")
        else:
            # JSON output: strip full_text if too long for practical use
            output = dict(result)
            if output.get("full_text") and len(output["full_text"]) > 5000:
                output["full_text"] = output["full_text"][:5000] + " [...truncated, use --text for more]"
            print(json.dumps(output, indent=2, ensure_ascii=False, default=str))


if __name__ == "__main__":
    main()
