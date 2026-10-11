# Audit reports

These are point-in-time code and documentation audits of this repository.
Findings carry stable IDs, C for codebase findings and D for documentation
findings, and those IDs are cited from commit messages
(e.g. "opc: ... (C53, C211)") to link each remediation commit back to the
finding it addresses. The reports describe the code as it stood on their
audit date; they are historical records and are not updated as findings
are fixed. Later reports re-verify earlier findings and record their
status at that date.

| Report | IDs |
|---|---|
| [codebase-audit-2026-07-07.md](codebase-audit-2026-07-07.md) | C1–C169 |
| [codebase-audit-2026-07-11.md](codebase-audit-2026-07-11.md) | C170–C235 |
| [codebase-audit-2026-07-26.md](codebase-audit-2026-07-26.md) | C236–C359 |
| [codebase-audit-2026-07-27.md](codebase-audit-2026-07-27.md) | C360–C582 |
| [docs-audit-2026-07-07.md](docs-audit-2026-07-07.md) | D1–D30 |
| [docs-audit-2026-07-26.md](docs-audit-2026-07-26.md) | D31–D49 |
| [docs-audit-2026-10-11.md](docs-audit-2026-10-11.md) | D50–D97 |

C583 and later were assigned to defects found outside an audit, and are
defined only by the commits, tests and comments that cite them. A new
finding takes the next number after the highest one cited anywhere in the
repository or its history.
