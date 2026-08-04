package preview

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	DefaultCSVTableWidth    = 80
	DefaultCSVColumnWidth   = 30
	minimumCSVColumnWidth   = 4
	minimumCSVTerminalWidth = 20
)

type CSVTableOptions struct {
	Width          int
	ColumnOffset   int
	MaxColumnWidth int
}

// RenderCSVTable converts structured CSV data into a width-bounded terminal
// table. ColumnOffset allows a viewport to scroll wide tables horizontally.
func RenderCSVTable(preview *CSVPreview, options CSVTableOptions) string {
	if preview == nil || len(preview.Rows) == 0 {
		return "(empty CSV)"
	}
	width := options.Width
	if width <= 0 {
		width = DefaultCSVTableWidth
	}
	width = max(width, minimumCSVTerminalWidth)
	maximumColumnWidth := options.MaxColumnWidth
	if maximumColumnWidth <= 0 {
		maximumColumnWidth = DefaultCSVColumnWidth
	}
	columnCount := preview.Metadata.ColumnCount
	if columnCount == 0 {
		columnCount = maxColumns(preview.Rows)
	}
	offset := min(max(options.ColumnOffset, 0), max(columnCount-1, 0))
	allWidths := csvColumnWidths(preview.Rows, columnCount, maximumColumnWidth)
	columns, widths := visibleCSVColumns(allWidths, offset, width)

	var table strings.Builder
	writeCSVBorder(&table, widths, '-')
	for rowIndex, row := range preview.Rows {
		table.WriteString("| ")
		for index, column := range columns {
			if index != 0 {
				table.WriteString(" | ")
			}
			value := ""
			if column < len(row) {
				value = row[column]
			}
			table.WriteString(csvTableCell(value, widths[index]))
		}
		table.WriteString(" |\n")
		if rowIndex == 0 {
			writeCSVBorder(&table, widths, '=')
		}
	}
	writeCSVBorder(&table, widths, '-')

	firstColumn := offset + 1
	lastColumn := offset + len(columns)
	fmt.Fprintf(&table, "Columns %d-%d of %d | Rows 1-%d of %d", firstColumn, lastColumn, columnCount, len(preview.Rows), preview.Metadata.RowCount)
	if offset > 0 {
		table.WriteString(" | more left")
	}
	if lastColumn < columnCount {
		table.WriteString(" | more right")
	}
	if preview.CellsTruncated {
		table.WriteString(" | cells shortened")
	}
	return table.String()
}

func csvColumnWidths(rows [][]string, columnCount, maximum int) []int {
	widths := make([]int, columnCount)
	for _, row := range rows {
		for column, value := range row {
			if column >= columnCount {
				break
			}
			widths[column] = max(widths[column], runewidth.StringWidth(value))
		}
	}
	for index := range widths {
		widths[index] = min(max(widths[index], minimumCSVColumnWidth), maximum)
	}
	return widths
}

func maxColumns(rows [][]string) int {
	maximum := 0
	for _, row := range rows {
		maximum = max(maximum, len(row))
	}
	return maximum
}

func visibleCSVColumns(allWidths []int, offset, terminalWidth int) ([]int, []int) {
	columns := make([]int, 0, len(allWidths)-offset)
	widths := make([]int, 0, len(allWidths)-offset)
	used := 1
	for column := offset; column < len(allWidths); column++ {
		columnWidth := allWidths[column]
		if used+columnWidth+3 > terminalWidth {
			if len(columns) == 0 {
				columns = append(columns, column)
				widths = append(widths, max(1, terminalWidth-4))
			}
			break
		}
		columns = append(columns, column)
		widths = append(widths, columnWidth)
		used += columnWidth + 3
	}
	return columns, widths
}

func writeCSVBorder(table *strings.Builder, widths []int, fill byte) {
	table.WriteByte('+')
	for _, width := range widths {
		table.WriteString(strings.Repeat(string(fill), width+2))
		table.WriteByte('+')
	}
	table.WriteByte('\n')
}

func csvTableCell(value string, width int) string {
	value = runewidth.Truncate(value, width, "...")
	return runewidth.FillRight(value, width)
}
