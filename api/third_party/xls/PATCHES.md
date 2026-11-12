# Patches to github.com/extrame/xls v0.0.1

Vendored copy of github.com/extrame/xls v0.0.1 (see LICENSE). Test files and the sample Table.xls were removed.

Change in `col.go`, `XfRk.String`:

- Upstream treated every number cell with a user-defined format (id >= 164) as a date and returned an RFC3339 timestamp, which corrupted prices (`"Rp" #,##0`) and identifiers (`0`).
- Now the date branch is taken only when the format is a date format: a custom format string containing d, m, y, h or s outside quoted text, `[..]` sections and backslash/underscore/asterisk escapes (`isDateFormat`), or a built-in date/time id (14-22, 27-36, 45-47, 50-58; `isBuiltinDateFormat`). Date cells are returned as RFC3339 (upstream returned only "2006.01" for some built-in ids, dropping the day).
- Every other number is returned as a plain number (`RK.String`, no exponent, no trailing ".0").

Change in `col.go` / `worksheet.go`, formula cells:

- Upstream `FormulaCol.String` returned the literal "FormulaCol". It now returns the cached result from the FORMULA record: a double is formatted like numeric cells (date formats via the cell's XF, otherwise plain `FormatFloat 'f' -1`); bytes 6-7 = 0xFFFF marks a special result (0 string, 1 bool -> "TRUE"/"FALSE", 2 error and 3 empty -> "").
- The text of a string result is read from the STRING record (0x207) that follows the FORMULA record (`WorkSheet.lastFormula`). Very long strings continued in CONTINUE records are truncated.
- `FormulaCol` also lacked `Row`/`FirstCol`/`LastCol` (its `Col` sits inside `Header`), so upstream never registered formula cells at all; these methods were added.
