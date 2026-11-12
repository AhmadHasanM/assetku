package importer

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

// ErrUnsupportedFormat is returned for file extensions other than csv, xlsx, xls.
var ErrUnsupportedFormat = errors.New("unsupported file format")

// ErrTooManyRows is returned by ReadRows when the file holds more non-blank
// rows than allowed. Reading stops as soon as the limit is passed.
var ErrTooManyRows = errors.New("too many rows")

// Unzip limits for xlsx: the total decompressed size, and the worksheet size
// above which excelize spills to a temp file instead of memory.
const (
	xlsxUnzipLimit    = 200 << 20
	xlsxUnzipXMLLimit = 64 << 20
)

// Row is a non-blank row of trimmed cells and its 1-based row number in the
// sheet (for csv, the line where the record starts).
type Row struct {
	Num   int
	Cells []string
}

// rowSink collects non-blank rows and enforces the limit.
type rowSink struct {
	max  int
	rows []Row
}

// add trims cells and keeps the row unless it is blank. It returns
// ErrTooManyRows once more than max (when > 0) non-blank rows were seen.
func (s *rowSink) add(num int, cells []string) error {
	blank := true
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
		if cells[i] != "" {
			blank = false
		}
	}
	if blank {
		return nil
	}
	if s.max > 0 && len(s.rows) == s.max {
		return ErrTooManyRows
	}
	s.rows = append(s.rows, Row{Num: num, Cells: cells})
	return nil
}

// ReadRows streams the first sheet (or the csv) and returns its non-blank rows
// with their row numbers, choosing the parser by the extension of name
// (case-insensitive). With maxRows > 0 it stops and returns ErrTooManyRows
// once more than maxRows non-blank rows were read.
func ReadRows(name string, r io.Reader, maxRows int) ([]Row, error) {
	sink := &rowSink{max: maxRows}
	var err error
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		err = readCSV(r, sink)
	case ".xlsx":
		err = readXLSX(r, sink)
	case ".xls":
		err = readXLS(r, sink)
	default:
		return nil, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, err
	}
	return sink.rows, nil
}

// ReadFile reads the whole first sheet (or the csv) into rows of trimmed
// strings; rows[i] is sheet row i+1, and blank rows are empty.
func ReadFile(name string, r io.Reader) ([][]string, error) {
	rows, err := ReadRows(name, r, 0)
	if err != nil {
		return nil, err
	}
	var out [][]string
	for _, row := range rows {
		for len(out) < row.Num-1 {
			out = append(out, []string{})
		}
		out = append(out, row.Cells)
	}
	return out, nil
}

func readCSV(r io.Reader, sink *rowSink) error {
	br := bufio.NewReaderSize(r, 64<<10)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte("\xef\xbb\xbf")) {
		_, _ = br.Discard(3)
	}
	// Pick the separator from the first non-blank line, which must fit in
	// the reader's buffer (64 KB) to be inspected; otherwise ',' is used.
	comma := ','
	if head, _ := br.Peek(br.Size()); len(head) > 0 {
		for rest := string(head); rest != ""; {
			line, after, found := strings.Cut(rest, "\n")
			rest = after
			if strings.TrimSpace(line) == "" && found {
				continue
			}
			if strings.Count(line, ";") > strings.Count(line, ",") {
				comma = ';'
			}
			break
		}
	}
	cr := csv.NewReader(br)
	cr.Comma = comma
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read csv: %w", err)
		}
		line, _ := cr.FieldPos(0)
		if err := sink.add(line, rec); err != nil {
			return err
		}
	}
}

func readXLSX(r io.Reader, sink *rowSink) error {
	f, err := excelize.OpenReader(r, excelize.Options{
		UnzipSizeLimit: xlsxUnzipLimit, UnzipXMLSizeLimit: xlsxUnzipXMLLimit,
	})
	if err != nil {
		return fmt.Errorf("read xlsx: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return errors.New("read xlsx: no sheets")
	}
	rows, err := f.Rows(sheets[0])
	if err != nil {
		return fmt.Errorf("read xlsx: %w", err)
	}
	defer rows.Close()
	for num := 1; rows.Next(); num++ {
		cells, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return fmt.Errorf("read xlsx: %w", err)
		}
		if err := sink.add(num, cells); err != nil {
			return err
		}
	}
	if err := rows.Error(); err != nil {
		return fmt.Errorf("read xlsx: %w", err)
	}
	return nil
}

func readXLS(r io.Reader, sink *rowSink) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("read xls: %v", p)
		}
	}()
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	wb, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return fmt.Errorf("read xls: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return errors.New("read xls: no sheets")
	}
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := xlsRow(sheet, i)
		if row == nil {
			continue
		}
		out := make([]string, row.LastCol()+1)
		for j := range out {
			out[j] = row.Col(j)
		}
		if err := sink.add(i+1, out); err != nil {
			return err
		}
	}
	return nil
}

// xlsRow returns row i, or nil when the row is absent (the library panics on
// rows that hold no cells).
func xlsRow(sheet *xls.WorkSheet, i int) (row *xls.Row) {
	defer func() {
		if recover() != nil {
			row = nil
		}
	}()
	return sheet.Row(i)
}
