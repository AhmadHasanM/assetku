# Patches to github.com/extrame/xls v0.0.1

Vendored copy of github.com/extrame/xls v0.0.1 (see LICENSE). Test files and the sample Table.xls were removed.

Change in `col.go`, `XfRk.String`:

- Upstream treated every number cell with a user-defined format (id >= 164) as a date and returned an RFC3339 timestamp, which corrupted prices (`"Rp" #,##0`) and identifiers (`0`).
- Now the date branch is taken only when the format is a date format: a custom format string containing d, m, y, h or s outside quoted text, `[..]` sections and backslash/underscore/asterisk escapes (`isDateFormat`), or a built-in date/time id (14-22, 27-36, 45-47, 50-58; `isBuiltinDateFormat`). Date cells are returned as RFC3339 (upstream returned only "2006.01" for some built-in ids, dropping the day).
- Every other number is returned as a plain number (`RK.String`, no exponent, no trailing ".0").
