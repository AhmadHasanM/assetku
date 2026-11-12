package importer

import (
	"encoding/csv"
	"io"
	"strconv"
)

// csvSafe neutralizes spreadsheet formula injection: a cell starting with
// = + - @ (or a tab/carriage return) is prefixed with a single quote.
func csvSafe(s string) string {
	if s != "" {
		switch s[0] {
		case '=', '+', '-', '@', '\t', '\r':
			return "'" + s
		}
	}
	return s
}

// WriteReport writes res as CSV: a "row,reason" section for skipped rows,
// then the unmatched organizations and the unrecognized values, separated by
// blank lines. Section headers are English.
func WriteReport(w io.Writer, res *Result) error {
	cw := csv.NewWriter(w)
	write := func(rec ...string) {
		for i := range rec {
			rec[i] = csvSafe(rec[i])
		}
		_ = cw.Write(rec)
	}
	write("row", "reason")
	for _, r := range res.SkippedRows {
		write(strconv.Itoa(r.Row), r.Reason)
	}
	write()
	write("unmatched_organization", "rows")
	for _, o := range res.UnmatchedOrganizations {
		write(o.Name, strconv.Itoa(o.Rows))
	}
	write()
	write("unrecognized_field", "value", "rows")
	for _, v := range res.UnrecognizedValues {
		write(v.Field, v.Value, strconv.Itoa(v.Rows))
	}
	cw.Flush()
	return cw.Error()
}
