package importer

import (
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

// ReadFile reads the first sheet (or the csv) into rows of trimmed strings,
// choosing the parser by the extension of name (case-insensitive).
func ReadFile(name string, r io.Reader) ([][]string, error) {
	var rows [][]string
	var err error
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		rows, err = readCSV(r)
	case ".xlsx":
		rows, err = readXLSX(r)
	case ".xls":
		rows, err = readXLS(r)
	default:
		return nil, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
	}
	return rows, nil
}

func readCSV(r io.Reader) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	comma := ','
	for rest := string(data); rest != ""; {
		line, after, _ := strings.Cut(rest, "\n")
		rest = after
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Count(line, ";") > strings.Count(line, ",") {
			comma = ';'
		}
		break
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = comma
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	return rows, nil
}

func readXLSX(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("read xlsx: no sheets")
	}
	rows, err := f.GetRows(sheets[0], excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}
	return rows, nil
}

func readXLS(r io.Reader) (rows [][]string, err error) {
	defer func() {
		if p := recover(); p != nil {
			rows, err = nil, fmt.Errorf("read xls: %v", p)
		}
	}()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	wb, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return nil, fmt.Errorf("read xls: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, errors.New("read xls: no sheets")
	}
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := xlsRow(sheet, i)
		if row == nil {
			rows = append(rows, nil)
			continue
		}
		out := make([]string, row.LastCol()+1)
		for j := range out {
			out[j] = row.Col(j)
		}
		rows = append(rows, out)
	}
	return rows, nil
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
