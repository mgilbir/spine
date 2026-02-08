#!/usr/bin/env python3
"""Extract structured implementation notes from MS-OI29500.

Parses the [MS-OI29500] PDF (Microsoft Office Implementation Information for
ISO/IEC 29500) and extracts:
  - Normative variation notes (Section 2.1.NNNN)
  - Per-note: section reference, element name, ISO section, application, note text
  - XML examples marked with [Example: ... end example]

Output: spec/testdata/msoi_notes.json

Requires: pymupdf (fitz)

Usage:
    python3 spec/extract_msoi.py
"""

import json
import os
import re
import sys

import fitz

# --- Configuration ---

SPEC_DIR = os.path.dirname(__file__)
OUTPUT_DIR = os.path.join(SPEC_DIR, "testdata")

PDF_CANDIDATES = [
    os.path.join(SPEC_DIR, "msoi29500", "MS-OI29500.pdf"),
]

# Section 2.1 contains normative variations (pages ~49-693 in the 250218 edition)
SECTION_2_START_PAGE = 48   # 0-indexed
SECTION_2_END_PAGE = 700    # generous upper bound

# Pattern for section headings: "2.1.NNNN  Part N Section XX.XX.XX, elementName (Description)"
SECTION_HEADING_RE = re.compile(
    r"^2\.1\.(\d+)\s+"
    r"Part\s+(\d)\s+Section\s+([\d.]+),\s+"
    r"(\S+)\s+"
    r"\(([^)]+)\)",
    re.MULTILINE,
)

# Alternative heading without element name: "2.1.NNNN  Part N Section XX.XX.XX (Description)"
SECTION_HEADING_ALT_RE = re.compile(
    r"^2\.1\.(\d+)\s+"
    r"Part\s+(\d)\s+Section\s+([\d.]+)\s*"
    r"\(([^)]+)\)",
    re.MULTILINE,
)

# Lettered sub-items: "a.  The standard states..."
SUB_ITEM_RE = re.compile(r"^([a-z])\.\s+", re.MULTILINE)

# Application mentions
APP_PATTERNS = {
    "Word": re.compile(r"\bWord\b"),
    "Excel": re.compile(r"\bExcel\b"),
    "PowerPoint": re.compile(r"\bPowerPoint\b"),
    "Office": re.compile(r"\bOffice\b"),
}

# Category detection patterns
CATEGORY_PATTERNS = [
    ("value_restriction", re.compile(
        r"restricts?\s+the\s+value|"
        r"value\s+(?:of|to)\s+be\s+at\s+(?:most|least)|"
        r"shall\s+(?:not\s+)?be\s+(?:greater|less|between)",
        re.IGNORECASE,
    )),
    ("default_value", re.compile(
        r"default\s+value\s+of|"
        r"uses?\s+a\s+default|"
        r"does\s+not\s+specify\s+a\s+default",
        re.IGNORECASE,
    )),
    ("required_attribute", re.compile(
        r"requires?\s+this\s+(?:attribute|element)|"
        r"(?:attribute|element)\s+is\s+(?:required|optional)|"
        r"implies\s+that\s+.*\s+is\s+optional",
        re.IGNORECASE,
    )),
    ("behavioral_deviation", re.compile(
        r"ignores?\s+this\s+(?:element|attribute)|"
        r"discards?\s+this|"
        r"does\s+not\s+(?:support|implement|use)|"
        r"will\s+not\s+(?:open|load|save)",
        re.IGNORECASE,
    )),
    ("format_handling", re.compile(
        r"(?:read|write|interpret).*percent|"
        r"trailing\s+percent\s+sign|"
        r"formatted\s+(?:as|with)",
        re.IGNORECASE,
    )),
]

# Page artifacts to strip
PAGE_ARTIFACT_RE = re.compile(r"\[MS-OI29500\].*?\n", re.IGNORECASE)
COPYRIGHT_RE = re.compile(r"Copyright\s+©.*?reserved\.?\s*\n?", re.IGNORECASE)
RELEASE_RE = re.compile(r"\d{1,2}/\d{1,2}/\d{4}\s*\n?")
PAGE_NUM_RE = re.compile(r"(?m)^\d{1,4}\s*(?:/\s*\d+)?\s*$")


def find_pdf():
    """Find the MS-OI29500 PDF."""
    for path in PDF_CANDIDATES:
        if os.path.exists(path):
            return path
    return None


def extract_full_text(doc, start_page, end_page):
    """Extract text from a page range, stripping page artifacts."""
    parts = []
    end = min(end_page, len(doc))
    for pg in range(start_page, end):
        text = doc[pg].get_text()
        # Strip common page artifacts
        text = PAGE_ARTIFACT_RE.sub("", text)
        text = COPYRIGHT_RE.sub("", text)
        text = RELEASE_RE.sub("", text)
        text = PAGE_NUM_RE.sub("", text)
        parts.append(text)
    return "\n".join(parts)


def detect_applications(text):
    """Detect which Office applications a note applies to."""
    apps = []
    for app, pattern in APP_PATTERNS.items():
        if pattern.search(text):
            apps.append(app)
    return apps if apps else ["Office"]


def categorize_note(text):
    """Categorize a note based on its content."""
    categories = []
    for category, pattern in CATEGORY_PATTERNS:
        if pattern.search(text):
            categories.append(category)
    return categories if categories else ["other"]


def extract_xml_examples(text):
    """Extract [Example: ... end example] blocks from note text."""
    examples = []
    idx = 0
    while idx < len(text):
        start = text.find("[Example:", idx)
        if start == -1:
            break
        end = text.find("end example]", start)
        if end == -1:
            break
        example_text = text[start:end + len("end example]")]
        # Try to extract XML from example
        xml_start = example_text.find("<")
        xml_end = example_text.rfind(">")
        xml_fragment = None
        if xml_start != -1 and xml_end > xml_start:
            xml_fragment = example_text[xml_start:xml_end + 1].strip()
        examples.append({
            "raw": example_text,
            "xml": xml_fragment,
        })
        idx = end + len("end example]")
    return examples


def split_sub_items(text):
    """Split note text into lettered sub-items."""
    # Find all sub-item markers
    markers = list(SUB_ITEM_RE.finditer(text))
    if not markers:
        return [{"letter": "", "text": text.strip()}]

    items = []
    for i, marker in enumerate(markers):
        start = marker.start()
        end = markers[i + 1].start() if i + 1 < len(markers) else len(text)
        item_text = text[start:end].strip()
        # Remove the "a. " prefix
        item_text = SUB_ITEM_RE.sub("", item_text, count=1).strip()
        items.append({
            "letter": marker.group(1),
            "text": item_text,
        })
    return items


def parse_sections(full_text):
    """Parse the full text into structured sections."""
    # Find all section headings
    headings = []

    for m in SECTION_HEADING_RE.finditer(full_text):
        headings.append({
            "pos": m.start(),
            "number": int(m.group(1)),
            "part": int(m.group(2)),
            "iso_section": m.group(3),
            "element_name": m.group(4),
            "description": m.group(5),
            "end_of_heading": m.end(),
        })

    for m in SECTION_HEADING_ALT_RE.finditer(full_text):
        num = int(m.group(1))
        # Skip if we already captured this section from the primary pattern
        if any(h["number"] == num for h in headings):
            continue
        headings.append({
            "pos": m.start(),
            "number": num,
            "part": int(m.group(2)),
            "iso_section": m.group(3),
            "element_name": "",
            "description": m.group(4),
            "end_of_heading": m.end(),
        })

    # Sort by position in text
    headings.sort(key=lambda h: h["pos"])

    # Extract body text between headings
    notes = []
    for i, heading in enumerate(headings):
        body_start = heading["end_of_heading"]
        body_end = headings[i + 1]["pos"] if i + 1 < len(headings) else len(full_text)
        body = full_text[body_start:body_end].strip()

        # Split into sub-items
        sub_items = split_sub_items(body)

        # Detect applications and categories from the full body
        apps = detect_applications(body)
        categories = categorize_note(body)

        # Extract XML examples
        xml_examples = extract_xml_examples(body)

        note = {
            "section_number": f"2.1.{heading['number']}",
            "part": heading["part"],
            "iso_section": heading["iso_section"],
            "element_name": heading["element_name"],
            "description": heading["description"],
            "applications": apps,
            "categories": categories,
            "sub_items": sub_items,
        }

        if xml_examples:
            note["xml_examples"] = xml_examples

        notes.append(note)

    return notes


def compute_stats(notes):
    """Compute summary statistics."""
    stats = {
        "total_notes": len(notes),
        "total_sub_items": sum(len(n["sub_items"]) for n in notes),
        "notes_with_xml": sum(1 for n in notes if "xml_examples" in n),
        "by_part": {},
        "by_category": {},
        "by_application": {},
    }

    for note in notes:
        part = f"Part {note['part']}"
        stats["by_part"][part] = stats["by_part"].get(part, 0) + 1

        for cat in note["categories"]:
            stats["by_category"][cat] = stats["by_category"].get(cat, 0) + 1

        for app in note["applications"]:
            stats["by_application"][app] = stats["by_application"].get(app, 0) + 1

    return stats


def main():
    pdf_path = find_pdf()
    if pdf_path is None:
        print("Error: MS-OI29500.pdf not found", file=sys.stderr)
        print(f"  Looked in: {PDF_CANDIDATES}", file=sys.stderr)
        sys.exit(1)

    os.makedirs(OUTPUT_DIR, exist_ok=True)

    doc = fitz.open(pdf_path)
    print(f"Opened {pdf_path} ({len(doc)} pages)", file=sys.stderr)

    print("Extracting text from Section 2.1...", file=sys.stderr)
    full_text = extract_full_text(doc, SECTION_2_START_PAGE, SECTION_2_END_PAGE)
    doc.close()

    print("Parsing sections...", file=sys.stderr)
    notes = parse_sections(full_text)

    stats = compute_stats(notes)

    print(f"\nExtracted {stats['total_notes']} notes with {stats['total_sub_items']} sub-items", file=sys.stderr)
    print(f"Notes with XML examples: {stats['notes_with_xml']}", file=sys.stderr)
    print(f"\nBy part:", file=sys.stderr)
    for part, count in sorted(stats["by_part"].items()):
        print(f"  {part}: {count}", file=sys.stderr)
    print(f"\nBy category:", file=sys.stderr)
    for cat, count in sorted(stats["by_category"].items()):
        print(f"  {cat}: {count}", file=sys.stderr)
    print(f"\nBy application:", file=sys.stderr)
    for app, count in sorted(stats["by_application"].items()):
        print(f"  {app}: {count}", file=sys.stderr)

    output = {
        "source": "MS-OI29500",
        "description": "Microsoft Office Implementation Information for ISO/IEC 29500",
        "stats": stats,
        "notes": notes,
    }

    output_path = os.path.join(OUTPUT_DIR, "msoi_notes.json")
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(output, f, indent=2, ensure_ascii=False)

    print(f"\nWrote {output_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
