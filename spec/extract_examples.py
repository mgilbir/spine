#!/usr/bin/env python3
"""Extract XML examples from ISO/IEC 29500 PDFs (all four parts).

Extracts [Example: ... end example] blocks, classifies them, and writes
JSON test data files grouped by format (WML, SML, PML, DML, etc.).

Part 1: Main spec (WML §17, SML §18, PML §19, DML §20-21, Shared §22)
Part 2: OPC (Open Packaging Conventions)
Part 3: Markup Compatibility and Extensibility
Part 4: Transitional Migration Features (WML §14, SML §15, PML §16, DML §17-18, VML §19, Shared §20)

Supports both 2012 and 2016/2021 editions; prefers the latest available.

Requires: pymupdf (fitz)

Usage:
    python3 spec/extract_examples.py
"""

import json
import os
import re
import sys
import xml.etree.ElementTree as ET

import fitz

# --- Configuration ---

SPEC_DIR = os.path.dirname(__file__)
OUTPUT_DIR = os.path.join(SPEC_DIR, "testdata")

# PDF definitions: each entry lists candidates in preference order (latest first).
# "document" is the ISO document identifier for breadcrumbs.
PDFS = [
    {
        "candidates": [
            {
                "path": os.path.join(SPEC_DIR, "part1", "ISO_IEC_29500-1_2016.pdf"),
                "document": "ISO/IEC 29500-1:2016",
                "sections": {
                    "WML": {"prefix": "17", "start_page": 178, "end_page": 1531},
                    "SML": {"prefix": "18", "start_page": 1532, "end_page": 2524},
                    "PML": {"prefix": "19", "start_page": 2525, "end_page": 2727},
                    "DML": {"prefix": "20", "start_page": 2728, "end_page": 3607},
                    "SHARED": {"prefix": "22", "start_page": 3608, "end_page": 3813},
                },
            },
            {
                "path": os.path.join(SPEC_DIR, "part1", "ISO_IEC_29500-1_2012.pdf"),
                "document": "ISO/IEC 29500-1:2012",
                "sections": {
                    "WML": {"prefix": "17", "start_page": 178, "end_page": 1529},
                    "SML": {"prefix": "18", "start_page": 1529, "end_page": 2520},
                    "PML": {"prefix": "19", "start_page": 2520, "end_page": 2720},
                    "DML": {"prefix": "20", "start_page": 2720, "end_page": 3600},
                },
            },
        ],
        "part_number": 1,
    },
    {
        "candidates": [
            {
                "path": os.path.join(SPEC_DIR, "part2", "ISO_IEC_29500-2_2021.pdf"),
                "document": "ISO/IEC 29500-2:2021",
                "sections": {
                    "OPC": {"prefix": "9", "start_page": 0, "end_page": 9999},
                },
            },
            {
                "path": os.path.join(SPEC_DIR, "part2", "ISO_IEC_29500-2_2012.pdf"),
                "document": "ISO/IEC 29500-2:2012",
                "sections": {
                    "OPC": {"prefix": "9", "start_page": 0, "end_page": 9999},
                },
            },
        ],
        "part_number": 2,
    },
    {
        "candidates": [
            {
                "path": os.path.join(SPEC_DIR, "part3", "ISO_IEC_29500-3_2015.pdf"),
                "document": "ISO/IEC 29500-3:2015",
                "sections": {
                    "MC": {"prefix": "9", "start_page": 0, "end_page": 9999},
                },
            },
            {
                "path": os.path.join(SPEC_DIR, "part3", "ISO_IEC_29500-3_2012.pdf"),
                "document": "ISO/IEC 29500-3:2012",
                "sections": {
                    "MC": {"prefix": "9", "start_page": 0, "end_page": 9999},
                },
            },
        ],
        "part_number": 3,
    },
    {
        "candidates": [
            {
                "path": os.path.join(SPEC_DIR, "part4", "ISO_IEC_29500-4_2016.pdf"),
                "document": "ISO/IEC 29500-4:2016",
                "sections": {
                    "WML_T": {"prefix": "14", "start_page": 45, "end_page": 219},
                    "SML_T": {"prefix": "15", "start_page": 219, "end_page": 241},
                    "PML_T": {"prefix": "16", "start_page": 241, "end_page": 267},
                    "DML_T": {"prefix": "17", "start_page": 267, "end_page": 307},
                    "VML":   {"prefix": "19", "start_page": 307, "end_page": 873},
                    "SHARED_T": {"prefix": "20", "start_page": 873, "end_page": 880},
                },
            },
            {
                "path": os.path.join(SPEC_DIR, "part4", "ISO_IEC_29500-4_2012.pdf"),
                "document": "ISO/IEC 29500-4:2012",
                "sections": {
                    "WML_T": {"prefix": "14", "start_page": 45, "end_page": 218},
                    "SML_T": {"prefix": "15", "start_page": 218, "end_page": 239},
                    "PML_T": {"prefix": "16", "start_page": 239, "end_page": 267},
                    "DML_T": {"prefix": "17", "start_page": 267, "end_page": 307},
                    "VML":   {"prefix": "19", "start_page": 307, "end_page": 873},
                    "SHARED_T": {"prefix": "20", "start_page": 873, "end_page": 9999},
                },
            },
        ],
        "part_number": 4,
    },
]

# Map internal section keys to output format names for JSON files.
# Multiple keys can map to the same output format (they get merged).
FORMAT_OUTPUT_MAP = {
    "WML": "wml", "WML_T": "wml",
    "SML": "sml", "SML_T": "sml",
    "PML": "pml", "PML_T": "pml",
    "DML": "dml", "DML_T": "dml",
    "OPC": "opc",
    "MC": "mc",
    "VML": "vml",
    "SHARED": "shared", "SHARED_T": "shared",
}

# Section header pattern: "17.3.1.1\nelement (Description)"
SECTION_HEADER_RE = re.compile(
    r"(\d+\.\d+(?:\.\d+)*)\s*\n(\w[\w\s]*?)\s*\(([^)]+)\)"
)

# Common namespace prefixes used in spec examples
NS_MAP = {
    "w": "http://schemas.openxmlformats.org/wordprocessingml/2006/main",
    "r": "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
    "a": "http://schemas.openxmlformats.org/drawingml/2006/main",
    "p": "http://schemas.openxmlformats.org/presentationml/2006/main",
    "wp": "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing",
    "m": "http://schemas.openxmlformats.org/officeDocument/2006/math",
    "mc": "http://schemas.openxmlformats.org/markup-compatibility/2006",
    "v": "urn:schemas-microsoft-com:vml",
    "o": "urn:schemas-microsoft-com:office:office",
    "wne": "http://schemas.microsoft.com/office/word/2006/wordml",
    "sl": "http://schemas.openxmlformats.org/schemaLibrary/2006/main",
    "c": "http://schemas.openxmlformats.org/drawingml/2006/chart",
    "dgm": "http://schemas.openxmlformats.org/drawingml/2006/diagram",
    "xdr": "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing",
    "x": "http://schemas.openxmlformats.org/spreadsheetml/2006/main",
    "wsp": "http://schemas.microsoft.com/office/word/2010/wordprocessingShape",
    "xml": "http://www.w3.org/XML/1998/namespace",
    "w10": "urn:schemas-microsoft-com:office:word",
    "x14": "http://schemas.microsoft.com/office/spreadsheetml/2009/9/main",
}

# Page artifact pattern matches all four part numbers and all editions
PAGE_ARTIFACT_RE = re.compile(r"ISO/IEC 29500-[1-4]:\d{4}\(E\)\s*")
COPYRIGHT_RE = re.compile(r"©\s?ISO/IEC \d{4} – All rights reserved\s*")
PAGE_NUM_RE = re.compile(r"(?m)^\d{1,4}\s*$")


def extract_text_by_page(doc, start_page, end_page):
    """Extract text from each page in range, returning list of (page_num, text)."""
    pages = []
    end = min(end_page, len(doc))
    for pg in range(start_page, end):
        pages.append((pg + 1, doc[pg].get_text()))  # 1-indexed page numbers
    return pages


# Pattern for ISO 2015+ EXAMPLE blocks (used in Part 2:2021, Part 3:2015)
EXAMPLE_BLOCK_RE = re.compile(r"EXAMPLE(?:\s+\d+)?\s*\t?\n")


def find_examples(pages):
    """Find all [Example: ... end example] blocks, handling multi-page spans.

    Also finds EXAMPLE blocks used in newer ISO editions (2015+).

    Returns list of dicts with keys: text, start_page.
    """
    examples = []
    buffer = ""
    buffer_page = None

    for page_num, text in pages:
        if buffer:
            # We're continuing a multi-page example
            end_idx = text.find("end example]")
            if end_idx != -1:
                buffer += " " + text[: end_idx + len("end example]")]
                examples.append({"text": buffer, "start_page": buffer_page})
                buffer = ""
                buffer_page = None
                # Continue scanning rest of this page
                remaining = text[end_idx + len("end example]") :]
                for ex in _find_examples_in_text(remaining, page_num):
                    if ex.get("_open"):
                        buffer = ex["text"]
                        buffer_page = page_num
                    else:
                        examples.append(ex)
            else:
                # Example continues to next page
                buffer += " " + text
        else:
            for ex in _find_examples_in_text(text, page_num):
                if ex.get("_open"):
                    buffer = ex["text"]
                    buffer_page = page_num
                else:
                    examples.append(ex)

    if buffer:
        print(f"  Warning: unclosed example starting at page {buffer_page}", file=sys.stderr)

    # Also find EXAMPLE blocks (newer ISO format)
    for page_num, text in pages:
        for ex in _find_example_blocks(text, page_num):
            examples.append(ex)

    return examples


def _find_examples_in_text(text, page_num):
    """Find examples within a single page's text. Yields dicts."""
    idx = 0
    while idx < len(text):
        start_idx = text.find("[Example:", idx)
        if start_idx == -1:
            break

        end_idx = text.find("end example]", start_idx)
        if end_idx == -1:
            yield {"text": text[start_idx:], "start_page": page_num, "_open": True}
            return
        else:
            full = text[start_idx : end_idx + len("end example]")]
            yield {"text": full, "start_page": page_num}
            idx = end_idx + len("end example]")


def _find_example_blocks(text, page_num):
    """Find EXAMPLE blocks (ISO 2015+ format) within a single page's text.

    These use "EXAMPLE" or "EXAMPLE N" as a heading, with content following
    until the next section heading or another EXAMPLE marker. Only yields
    blocks that contain XML (at least one '<' character).
    """
    for m in EXAMPLE_BLOCK_RE.finditer(text):
        start = m.end()
        # Find end: next EXAMPLE block, or next section heading pattern
        next_example = EXAMPLE_BLOCK_RE.search(text, start)
        # Also look for section heading patterns (e.g. "6.5.2\n")
        next_section = re.search(r"\n\d+\.\d+(?:\.\d+)*\s*\t?\n", text[start:])

        end = len(text)
        if next_example:
            end = min(end, next_example.start())
        if next_section:
            end = min(end, start + next_section.start())

        content = text[start:end].strip()
        if "<" in content:
            # Wrap in [Example: ... end example] format for uniform processing
            yield {"text": f"[Example: {content} end example]", "start_page": page_num}


def strip_page_artifacts(text):
    """Remove PDF page headers/footers/page numbers from multi-page examples."""
    text = PAGE_ARTIFACT_RE.sub("", text)
    text = COPYRIGHT_RE.sub("", text)
    text = PAGE_NUM_RE.sub("", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text


def extract_xml_fragment(example_text):
    """Extract the XML fragment from an example text block."""
    text = example_text
    if text.startswith("[Example:"):
        text = text[len("[Example:") :]
    if text.endswith("end example]"):
        text = text[: -len("end example]")]

    text = strip_page_artifacts(text)
    text = text.strip()

    xml_start = None
    xml_end = None

    for i, ch in enumerate(text):
        if ch == "<" and i + 1 < len(text) and (text[i + 1].isalpha() or text[i + 1] in "/?!"):
            line_start = text.rfind("\n", 0, i)
            if line_start == -1:
                line_start = 0
            prefix = text[line_start:i].strip()

            if prefix and re.search(r"\w$", prefix):
                close_gt = text.find(">", i)
                if close_gt != -1:
                    after_gt = text[close_gt + 1 : close_gt + 50].strip()
                    if after_gt and not after_gt.startswith("<") and re.match(r"[a-z]", after_gt):
                        continue
                    xml_start = i
                    break
                continue
            xml_start = i
            break

    if xml_start is None:
        return None

    xml_end = text.rfind(">")
    if xml_end is None or xml_end <= xml_start:
        return None

    fragment = text[xml_start : xml_end + 1]
    fragment = _trim_trailing_prose(fragment)

    return fragment.strip() if fragment.strip() else None


def _trim_trailing_prose(fragment):
    """Remove trailing prose that may have been included after the XML."""
    lines = fragment.split("\n")
    result_lines = []
    found_xml = False
    after_xml_lines = 0

    for line in lines:
        stripped = line.strip()
        if "<" in stripped or ">" in stripped:
            found_xml = True
            after_xml_lines = 0
            result_lines.append(line)
        elif found_xml and stripped:
            after_xml_lines += 1
            if after_xml_lines > 2:
                break
            if len(stripped) < 100 and not stripped.endswith("."):
                result_lines.append(line)
            else:
                break
        else:
            result_lines.append(line)

    return "\n".join(result_lines)


def detect_root_element(xml_fragment):
    """Detect the root element name and namespace prefix from an XML fragment."""
    if not xml_fragment:
        return None, ""

    m = re.match(r"\s*<\??(?:!--.*?-->\s*<)?([a-zA-Z_][\w]*(?::[\w]+)?)", xml_fragment, re.DOTALL)
    if not m:
        return None, ""

    tag = m.group(1)
    if ":" in tag:
        prefix, local = tag.split(":", 1)
        return local, prefix
    return tag, ""


def has_ellipsis(text):
    """Check if text contains ellipsis markers."""
    return "\u2026" in text or "…" in text or "..." in text


def strip_ellipsis(xml_fragment):
    """Strip ellipsis markers from XML and attempt to make it parseable."""
    if not xml_fragment:
        return None

    result = xml_fragment
    result = result.replace("\u2026", "")
    result = result.replace("…", "")
    result = re.sub(r"\.{3,}", "", result)
    result = re.sub(r'\s+\w+(?::\w+)?=""\s*', " ", result)
    result = re.sub(r"\s+>", ">", result)
    result = re.sub(r"\s+/>", "/>", result)
    return result.strip()


def wrap_with_namespaces(xml_fragment, format_type):
    """Wrap an XML fragment with namespace declarations for parsing."""
    ns_attrs = []
    for prefix, uri in NS_MAP.items():
        ns_attrs.append(f'xmlns:{prefix}="{uri}"')

    ns_str = " ".join(ns_attrs)
    return f"<wrapper {ns_str}>{xml_fragment}</wrapper>"


def try_parse_xml(xml_fragment, format_type):
    """Try to parse an XML fragment. Returns True if successful."""
    if not xml_fragment:
        return False

    wrapped = wrap_with_namespaces(xml_fragment, format_type)
    try:
        ET.fromstring(wrapped)
        return True
    except ET.ParseError:
        try:
            ET.fromstring(xml_fragment)
            return True
        except ET.ParseError:
            return False


def classify_example(xml_fragment, format_type):
    """Classify an example XML fragment."""
    if xml_fragment is None:
        return "no_xml", None

    has_ell = has_ellipsis(xml_fragment)

    if not has_ell:
        if try_parse_xml(xml_fragment, format_type):
            return "clean", None
        else:
            return "malformed", None
    else:
        stripped = strip_ellipsis(xml_fragment)
        if stripped and try_parse_xml(stripped, format_type):
            return "ellipsis_strippable", stripped
        else:
            return "has_ellipsis", None


def track_section(text, current_section, current_element, current_desc):
    """Track the current section header from page text."""
    matches = list(SECTION_HEADER_RE.finditer(text))
    if matches:
        last = matches[-1]
        return last.group(1), last.group(2).strip(), last.group(3)
    return current_section, current_element, current_desc


def make_example_id(format_key, section, counter):
    """Generate a unique example ID."""
    section_slug = section.replace(".", "_")
    return f"{format_key.lower()}_{section_slug}_{counter:03d}"


def process_section(doc, section_config, format_key, document_id):
    """Process a section of the PDF and extract examples."""
    start = section_config["start_page"]
    end = section_config["end_page"]

    print(f"  Processing {format_key} (pages {start + 1}-{min(end, len(doc))})...", file=sys.stderr)

    pages = extract_text_by_page(doc, start, end)

    # First pass: track section headers per page
    section_at_page = {}
    current_section = section_config["prefix"]
    current_element = ""
    current_desc = ""

    for page_num, text in pages:
        current_section, current_element, current_desc = track_section(
            text, current_section, current_element, current_desc
        )
        section_at_page[page_num] = (current_section, current_element, current_desc)

    # Second pass: extract examples
    raw_examples = find_examples(pages)

    examples = []
    section_counters = {}

    for raw_ex in raw_examples:
        page = raw_ex["start_page"]
        section, element_name, desc = section_at_page.get(
            page, (section_config["prefix"], "", "")
        )

        if not element_name:
            for p in range(page, start, -1):
                if p in section_at_page and section_at_page[p][1]:
                    section, element_name, desc = section_at_page[p]
                    break

        key = section
        section_counters[key] = section_counters.get(key, 0) + 1

        xml_fragment = extract_xml_fragment(raw_ex["text"])
        root_element, ns_prefix = detect_root_element(xml_fragment)
        classification, xml_stripped = classify_example(xml_fragment, format_key)

        example_id = make_example_id(format_key, section, section_counters[key])

        examples.append(
            {
                "id": example_id,
                "section": section,
                "element_name": element_name,
                "description": desc,
                "page": page,
                "document": document_id,
                "root_element": root_element,
                "ns_prefix": ns_prefix,
                "classification": classification,
                "xml": xml_fragment,
                "xml_stripped": xml_stripped,
            }
        )

    return examples


def print_stats(examples, label):
    """Print classification statistics."""
    stats = {}
    for ex in examples:
        c = ex["classification"]
        stats[c] = stats.get(c, 0) + 1

    print(f"\n  {label} statistics:", file=sys.stderr)
    print(f"    Total examples: {len(examples)}", file=sys.stderr)
    for cls in ["clean", "ellipsis_strippable", "has_ellipsis", "no_xml", "malformed"]:
        count = stats.get(cls, 0)
        pct = 100 * count / len(examples) if examples else 0
        print(f"    {cls}: {count} ({pct:.1f}%)", file=sys.stderr)


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # Collect all examples grouped by output format
    all_examples = {}  # output_format -> list of examples

    for pdf_group in PDFS:
        part_number = pdf_group["part_number"]

        # Find the first available candidate PDF
        selected = None
        for candidate in pdf_group["candidates"]:
            if os.path.exists(candidate["path"]):
                selected = candidate
                break

        if selected is None:
            tried = [c["path"] for c in pdf_group["candidates"]]
            print(f"Warning: No PDF found for Part {part_number}, tried: {tried}", file=sys.stderr)
            continue

        pdf_path = selected["path"]
        document_id = selected["document"]
        sections = selected["sections"]

        doc = fitz.open(pdf_path)
        print(f"\nOpened {document_id} ({len(doc)} pages)", file=sys.stderr)

        for section_key, section_config in sections.items():
            output_format = FORMAT_OUTPUT_MAP[section_key]
            examples = process_section(doc, section_config, section_key, document_id)

            if output_format not in all_examples:
                all_examples[output_format] = []
            all_examples[output_format].extend(examples)

            print_stats(examples, f"{document_id} {section_key}")

        doc.close()

    # Write output files
    print("\n--- Output ---", file=sys.stderr)
    for output_format, examples in sorted(all_examples.items()):
        # Determine document IDs present
        doc_ids = sorted(set(ex["document"] for ex in examples))

        output = {
            "documents": doc_ids,
            "format": output_format.upper(),
            "examples": examples,
        }

        output_path = os.path.join(OUTPUT_DIR, f"{output_format}_examples.json")
        with open(output_path, "w", encoding="utf-8") as f:
            json.dump(output, f, indent=2, ensure_ascii=False)

        print(f"  {output_path}: {len(examples)} examples", file=sys.stderr)

    print("\nDone.", file=sys.stderr)


if __name__ == "__main__":
    main()
