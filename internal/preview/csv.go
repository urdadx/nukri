package preview

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) renderCSV(ctx context.Context, path string) (*CSVPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV: %w", err)
	}
	defer file.Close()

	limited := &io.LimitedReader{R: file, N: s.maxCSVBytes + 1}
	reader := csv.NewReader(limited)
	reader.FieldsPerRecord = -1
	delimiter := ','
	if strings.EqualFold(filepath.Ext(path), ".tsv") {
		delimiter = '\t'
	}
	reader.Comma = delimiter

	rows := make([][]string, 0, s.maxCSVRows)
	totalRows := 0
	columnCount := 0
	columnsTruncated := false
	cellsTruncated := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV: %w", err)
		}
		totalRows++
		columnCount = max(columnCount, len(record))
		if len(record) > s.maxCSVColumns {
			columnsTruncated = true
		}
		if len(rows) >= s.maxCSVRows {
			continue
		}
		visibleColumns := min(len(record), s.maxCSVColumns)
		row := make([]string, visibleColumns)
		for index := range visibleColumns {
			row[index], cellsTruncated = csvCell(record[index], s.maxCSVCellRunes, cellsTruncated)
		}
		rows = append(rows, row)
	}
	if limited.N <= 0 {
		return nil, ErrOutputTooLarge
	}
	return &CSVPreview{
		Rows: rows,
		Metadata: CSVMetadata{
			RowCount: totalRows, ColumnCount: columnCount, Delimiter: delimiter,
		},
		RowsTruncated:    totalRows > len(rows),
		ColumnsTruncated: columnsTruncated,
		CellsTruncated:   cellsTruncated,
	}, nil
}

func csvCell(value string, maximumRunes int, alreadyTruncated bool) (string, bool) {
	value = strings.ToValidUTF8(value, "\uFFFD")
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, value)
	value = safeText(value)
	runes := []rune(value)
	if len(runes) > maximumRunes {
		return string(runes[:maximumRunes]), true
	}
	return value, alreadyTruncated
}
