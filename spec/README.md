# Spec Directory

This directory contains extraction scripts and test data derived from the ISO/IEC 29500 (Office Open XML) specification and the Microsoft Office implementation notes.

The spec PDFs and schemas are copyrighted and not included in this repository. Follow the instructions below to download them.

## Downloading the ISO/IEC 29500 Spec Files

1. Go to https://www.iso.org/search.html and search for `ISO/IEC 29500-`
2. Purchase/download the following documents (latest editions):
   - **ISO/IEC 29500-1:2016** - Fundamentals and Markup Language Reference
   - **ISO/IEC 29500-2:2021** - Open Packaging Conventions
   - **ISO/IEC 29500-3:2015** - Markup Compatibility and Extensibility
   - **ISO/IEC 29500-4:2016** - Transitional Migration Features

   The downloaded files will be named:
   ```
   ISO_IEC_29500-1_2016(en).pdf
   ISO_IEC_29500-1_2016(en)_einsert.zip
   ISO_IEC_29500-2_2021(en).pdf
   ISO_IEC_29500-2_2021(en).epub
   ISO_IEC_29500-3_2015(en).pdf
   ISO_IEC_29500-4_2016(en).pdf
   ISO_IEC_29500-4_2016(en)_einsert.zip
   ```

3. Create the target directories and place the PDFs (renaming to drop the `(en)` suffix):

   ```bash
   cd spec/
   mkdir -p part1 part2 part3 part4

   cp ISO_IEC_29500-1_2016\(en\).pdf part1/ISO_IEC_29500-1_2016.pdf
   cp ISO_IEC_29500-2_2021\(en\).pdf part2/ISO_IEC_29500-2_2021.pdf
   cp ISO_IEC_29500-2_2021\(en\).epub part2/ISO_IEC_29500-2_2021.epub
   cp ISO_IEC_29500-3_2015\(en\).pdf part3/ISO_IEC_29500-3_2015.pdf
   cp ISO_IEC_29500-4_2016\(en\).pdf part4/ISO_IEC_29500-4_2016.pdf
   ```

4. Extract the einsert ZIPs (these contain XSD schemas, RELAX NG schemas, geometries, and styles as nested ZIPs):

   ```bash
   # Part 1 einsert
   cd /tmp && mkdir einsert && cd einsert
   unzip /path/to/ISO_IEC_29500-1_2016\(en\)_einsert.zip

   # XSD schemas (Strict)
   mkdir -p /path/to/spec/part1/xsd
   unzip OfficeOpenXML-XMLSchema-Strict.zip -d /path/to/spec/part1/xsd

   # RELAX NG schemas
   mkdir -p /path/to/spec/part1/relaxng
   unzip OfficeOpenXML-RELAXNG-Strict.zip -d /path/to/spec/part1/relaxng

   # Geometry definitions
   mkdir -p /path/to/spec/part1/geometries
   unzip OfficeOpenXML-DrawingMLGeometries.zip -d /path/to/spec/part1/geometries

   # Spreadsheet styles
   mkdir -p /path/to/spec/part1/styles
   unzip OfficeOpenXML-SpreadsheetMLStyles.zip -d /path/to/spec/part1/styles

   # Part 4 einsert
   rm -f *.zip
   unzip /path/to/ISO_IEC_29500-4_2016\(en\)_einsert.zip

   # XSD schemas (Transitional)
   mkdir -p /path/to/spec/part4/xsd
   unzip OfficeOpenXML-XMLSchema-Transitional.zip -d /path/to/spec/part4/xsd

   # RELAX NG schemas
   mkdir -p /path/to/spec/part4/relaxng
   unzip OfficeOpenXML-RELAXNG-Transitional.zip -d /path/to/spec/part4/relaxng

   # Clean up
   cd / && rm -rf /tmp/einsert
   ```

## Downloading the MS-OI29500 Document

1. Go to https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oi29500/1fd4a662-8623-49c0-82f0-18fa91b413b8
2. Download the PDF and DOCX versions of `[MS-OI29500]`
3. Place them in the `msoi29500/` directory:

   ```bash
   mkdir -p spec/msoi29500
   cp \[MS-OI29500\].pdf spec/msoi29500/MS-OI29500.pdf
   cp \[MS-OI29500\]-250218.docx spec/msoi29500/MS-OI29500-250218.docx
   ```

## Expected Directory Layout

After setup, the directory should look like:

```
spec/
├── part1/                      # ISO/IEC 29500-1:2016 (Fundamentals)
│   ├── ISO_IEC_29500-1_2016.pdf
│   ├── xsd/                    # Strict XSD schemas (21 files)
│   ├── relaxng/                # Strict RELAX NG schemas (86 files)
│   ├── geometries/             # Preset shape/warp definitions (2 XMLs)
│   └── styles/                 # Preset cell/table styles (2 XMLs + 1 xlsx)
├── part2/                      # ISO/IEC 29500-2:2021 (OPC)
│   ├── ISO_IEC_29500-2_2021.pdf
│   ├── ISO_IEC_29500-2_2021.epub
│   └── xsd/                    # OPC package-layer schemas (see "Schema validation")
├── part3/                      # ISO/IEC 29500-3:2015 (MC)
│   └── ISO_IEC_29500-3_2015.pdf
├── part4/                      # ISO/IEC 29500-4:2016 (Transitional)
│   ├── ISO_IEC_29500-4_2016.pdf
│   ├── xsd/                    # Transitional XSD schemas (26 files)
│   └── relaxng/                # Transitional RELAX NG schemas (92 files)
├── msoi29500/                  # Microsoft Office Implementation Notes
│   ├── MS-OI29500.pdf
│   └── MS-OI29500-250218.docx
├── extract_examples.py         # Extract XML examples from ISO PDFs
├── extract_msoi.py             # Extract implementation notes from MS-OI29500
├── testdata/                   # Generated test data (committed)
├── spectest/                   # Go test harness
└── gen_spec/                   # Shape-type generator (committed, see below)
```

Only `part1/` to `part4/` and `msoi29500/` are gitignored (`spec/.gitignore`);
everything else here is tracked.

## gen_spec (committed, not part of the build)

`gen_spec/` is a tool derived from
[python-pptx](https://github.com/scanny/python-pptx)'s spec-generation
tooling (MIT-licensed; the license is vendored at `python-tests/LICENSE`).
It generates shape-type constants from a spec database. Its files are
committed (`git ls-files spec/gen_spec`), but nothing in the build or tests
depends on it. Its scripts still contain the upstream author's local paths
(`gen_spec/gen_spec.py`) and need adjusting before use.

## Schema validation

`internal/schemavalid` validates the parts spine authors against the
schemas in `SchemaDirs`: `spec/part4/xsd` (the Transitional markup, from the
Part 4 einsert above) and `spec/part2/xsd`, the OPC package-layer schemas
(content types, relationships, core properties). Neither the Part 2 files
listed above nor the einsert steps produce `part2/xsd`, and neither this
repository nor its history records where the maintainer's copy came from:
create `spec/part2/xsd` and put the Part 2 OPC `.xsd` files in it by hand.
`opc-coreProperties.xsd` is skipped on purpose, because it references Dublin
Core schemas that are not vendored. The tests also need a validator backend,
`xmllint` (`libxml2-utils`) or Python with `lxml`.

When the schemas or the validator are missing, the suite skips and says which.
Three environment variables enforce or regenerate spec-derived data:

| Variable | Effect |
|---|---|
| `SPINE_REQUIRE_SCHEMA` | Any value: a missing validator or missing schemas fails the test instead of skipping. Set it on a machine that is supposed to have both. |
| `SPINE_UPDATE_ORDER_TABLE` | Any value: `TestUpdateChildOrderTable` regenerates `internal/schemavalid/testdata/child_order.tsv` from `part2/xsd` and `part4/xsd`, and fails if they are absent. Commit the result: the child-order check reads the committed table, so it runs without the schemas, CI included. |
| `SPINE_SPEC_UPDATE_BASELINE` | Any value: the spec-example suites append their observed coverage rows to `spec/spectest/coverage_baseline.tsv.observed` instead of checking `coverage_baseline.tsv`; sort the rows into the baseline yourself (see `updateBaseline` in `spectest/coverage_ratchet.go`). Needs only the committed `spec/testdata/`, not the ISO files. |

## Running the Extraction Scripts

Requires Python 3 with `pymupdf`:

```bash
pip install pymupdf
```

Extract XML examples from the ISO spec PDFs:

```bash
python3 spec/extract_examples.py
```

Extract implementation notes from MS-OI29500:

```bash
python3 spec/extract_msoi.py
```

Output is written to `spec/testdata/`.
