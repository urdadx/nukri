package preview

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

/*
renderCSV parses the first maxCSVRows records for display, then counts the rest
with countCSVRecords instead of parsing them: the count is all the preview
needs, and scanning is several times faster than building every record. Only
the parsed part is bound by maxCSVBytes, so large files still get a preview.
*/
func (s *Service) renderCSV(ctx context.Context, path string) (*CSVPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV: %w", err)
	}
	defer file.Close()

	limited := &io.LimitedReader{R: file, N: s.maxCSVBytes + 1}
	buffered := bufio.NewReaderSize(limited, 64<<10)
	reader := csv.NewReader(buffered) // reuses buffered, so unread input stays there
	reader.FieldsPerRecord = -1
	delimiter := ','
	if strings.EqualFold(filepath.Ext(path), ".tsv") {
		delimiter = '\t'
	}
	reader.Comma = delimiter

	rows := make([][]string, 0, s.maxCSVRows)
	columnCount := 0
	cellsTruncated := false
	for len(rows) < s.maxCSVRows {
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
		columnCount = max(columnCount, len(record))
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

	// Count what follows: first the bytes already buffered, then the file.
	remaining, err := countCSVRecords(ctx, io.MultiReader(buffered, file), delimiter, csvCountByteLimit)
	if err != nil {
		return nil, err
	}
	columnCount = max(columnCount, remaining.maxFields)
	totalRows := len(rows) + remaining.records
	return &CSVPreview{
		Rows: rows,
		Metadata: CSVMetadata{
			RowCount: totalRows, RowCountAtLeast: remaining.partial, ColumnCount: columnCount, Delimiter: delimiter,
		},
		RowsTruncated:    totalRows > len(rows),
		ColumnsTruncated: columnCount > s.maxCSVColumns,
		CellsTruncated:   cellsTruncated,
	}, nil
}

// csvCountByteLimit bounds how much of a file is scanned to count its rows.
// Past it the count is a minimum, so a huge file can't hold up the preview.
const csvCountByteLimit = 64 << 20

type csvCount struct {
	records   int
	maxFields int
	partial   bool // stopped at the byte limit
}

/*
countCSVRecords counts the records in r, which must start at a record boundary,
and the most fields in any of them, without building them. Quotes are tracked
so delimiters and line breaks inside quoted fields don't count; a doubled quote
toggles twice and so needs no special case. Blank lines are skipped, as
encoding/csv skips them.

Lines without quotes, the common case, are counted with the standard library's
vectorized byte search; only lines with quotes are walked byte by byte.
*/
func countCSVRecords(ctx context.Context, r io.Reader, delimiter rune, limit int64) (csvCount, error) {
	var count csvCount
	if delimiter > 0x7f {
		delimiter = ',' // a multi-byte delimiter is not supported by this scan
	}
	scan := csvScanner{separator: byte(delimiter), fields: 1}
	reader := bufio.NewReaderSize(r, 256<<10)
	var scanned int64
	for lines := 0; ; lines++ {
		if lines%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}
		// A line longer than the buffer arrives in pieces (ErrBufferFull); the
		// scanner's state carries across them.
		line, err := reader.ReadSlice('\n')
		scan.feed(line, &count)
		scanned += int64(len(line))
		if err == io.EOF {
			scan.end(&count)
			return count, nil
		}
		if err != nil && err != bufio.ErrBufferFull {
			return count, err
		}
		if scanned >= limit {
			count.partial = true
			return count, nil
		}
	}
}

// csvScanner holds the state of countCSVRecords between pieces of input.
type csvScanner struct {
	separator byte
	fields    int  // fields so far in the current record
	content   bool // the current record has any content
	quoted    bool // inside a quoted field
}

func (s *csvScanner) feed(piece []byte, count *csvCount) {
	if !s.quoted && bytes.IndexByte(piece, '"') < 0 {
		// Fast path: at most one line break, at the end of the piece.
		body := bytes.TrimSuffix(piece, []byte{'\n'})
		s.fields += bytes.Count(body, []byte{s.separator})
		if len(bytes.TrimRight(body, "\r")) > 0 {
			s.content = true
		}
		if len(body) < len(piece) {
			s.endRecord(count)
		}
		return
	}
	for _, b := range piece {
		switch {
		case s.quoted:
			if b == '"' {
				s.quoted = false
			}
		case b == '"':
			s.quoted, s.content = true, true
		case b == s.separator:
			s.fields++
			s.content = true
		case b == '\n':
			s.endRecord(count)
		case b != '\r':
			s.content = true
		}
	}
}

func (s *csvScanner) endRecord(count *csvCount) {
	if s.content {
		count.records++
		count.maxFields = max(count.maxFields, s.fields)
	}
	s.fields, s.content = 1, false
}

// end finishes a final record that has no trailing line break.
func (s *csvScanner) end(count *csvCount) {
	s.endRecord(count)
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
