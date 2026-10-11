# Excel range rendering

This page is the sheet range profile: what `xlsxrender.PrepareRange`
draws and what fails it. It is for callers rendering cell ranges, who
need to know that ranges are always strict and which workbooks qualify.
It is part of the [rendering guide](../rendering.md), which has the quick
start and the entry points.

`xlsxrender.PrepareRange(ctx, sheet, "A1:D10", opts)` snapshots that logical range at 96 CSS
pixels per inch, without UI headers or print pagination. The first profile
requires the library's default stylesheet and explicit widths on every selected
column. Supply the Normal font through `opts.Fonts` (Calibri, 11 pt); substitution
is controlled by the caller. The native maximum digit advance is rounded to a
96-DPI pixel, then the [SpreadsheetML column width formula](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.spreadsheet.column)
is applied. Row heights use their explicit or sheet-default point sizes; the
created-sheet fallback is 15 pt. Hidden rows/columns are omitted. Grid lines
use a fixed light gray and respect `showGridLines`.

Plain ASCII strings may extend through blank neighbours inside the range.
General integers with fewer than ten decimal digits are right aligned; booleans
are centred. Supported cached formula values follow the same rules, without
evaluation. Other numbers, number formats, rich strings, alternate styles,
merges, conditional formatting, drawings and other worksheet features fail.
Horizontal/vertical overflow fails instead of silently clipping a value or
changing its formatting. Glyph overhang is clipped to the permitted cell span.

Preparation checks original worksheet/styles/shared-string XML, bounds source
indexing and range size, and creates no missing cells. The snapshot includes
unsaved values. Original source checks remain conservative for pending edits.
Native font/grid metrics do not promise Excel pixel identity.
