package preview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
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
	table.WriteString(drawCSVTable(preview.Rows, columns, widths))
	table.WriteByte('\n')

	firstColumn := offset + 1
	lastColumn := offset + len(columns)
	fmt.Fprintf(&table, "Columns %d-%d of %d | Rows 1-%d of %s", firstColumn, lastColumn, columnCount, len(preview.Rows), preview.Metadata.rowCountLabel())
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
			widths[column] = max(widths[column], displayWidth(value, runewidth.StringWidth))
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

// Table colors are ANSI palette indexes, so they follow the terminal theme.
var (
	csvBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	csvHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

/*
drawCSVTable draws rows (the first one as the header) restricted to columns,
each cell fitted to its width in widths.

Lip Gloss draws the borders and header, but body rows are built here: styling
every cell through Lip Gloss cost about 25µs a cell, over a second for a large
archive listing. Body cells carry no styling beyond padding and alignment, so
each row is the border character, styled once, between padded cells, exactly as
Lip Gloss would write it.
*/
func drawCSVTable(rows [][]string, columns, widths []int) string {
	numeric := numericColumns(rows, columns)
	header := make([]string, len(columns))
	for index, column := range columns {
		if column < len(rows[0]) {
			header[index] = runewidth.Truncate(rows[0][column], widths[index], "…")
		}
	}
	frame := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(csvBorderStyle).
		BorderRow(false).
		Headers(header...).
		StyleFunc(func(row, column int) lipgloss.Style {
			// Width includes the one-cell padding on each side.
			style := lipgloss.NewStyle().Padding(0, 1).Width(widths[column] + 2)
			if numeric[column] {
				style = style.Align(lipgloss.Right)
			}
			if row == table.HeaderRow {
				return style.Inherit(csvHeaderStyle)
			}
			return style
		}).
		Render()
	// frame is the top border, header, header separator and bottom border.
	bottom := strings.LastIndexByte(frame, '\n')
	if bottom < 0 || len(rows) < 2 {
		return frame
	}
	separator := csvBorderStyle.Render(lipgloss.RoundedBorder().Left)
	var body strings.Builder
	for _, row := range rows[1:] {
		body.WriteString(separator)
		for index, column := range columns {
			value := ""
			if column < len(row) {
				value = fitCell(row[column], widths[index])
			}
			gap := strings.Repeat(" ", max(0, widths[index]-displayWidth(value, ansi.StringWidth)))
			body.WriteByte(' ')
			if numeric[index] {
				body.WriteString(gap + value)
			} else {
				body.WriteString(value + gap)
			}
			body.WriteByte(' ')
			body.WriteString(separator)
		}
		body.WriteByte('\n')
	}
	return frame[:bottom+1] + body.String() + frame[bottom+1:]
}

// numericColumns reports, for each of columns, whether every non-empty body
// cell is a number. Those columns are right-aligned so digits line up.
func numericColumns(rows [][]string, columns []int) []bool {
	numeric := make([]bool, len(columns))
	for index, column := range columns {
		seen := false
		numeric[index] = true
		for _, row := range rows[1:] {
			if column >= len(row) || strings.TrimSpace(row[column]) == "" {
				continue
			}
			seen = true
			if _, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(row[column]), ",", ""), 64); err != nil {
				numeric[index] = false
				break
			}
		}
		numeric[index] = numeric[index] && seen
	}
	return numeric
}

// displayWidth is the cell width of value. Plain printable ASCII, the usual case
// for file names and data, is one cell per byte; anything else goes to measure.
func displayWidth(value string, measure func(string) int) int {
	if isPlainASCII(value) {
		return len(value)
	}
	return measure(value)
}

func isPlainASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 || value[index] < 0x20 {
			return false
		}
	}
	return true
}

// fitCell truncates value to width cells, marking the cut with "…".
func fitCell(value string, width int) string {
	if len(value) <= width && isPlainASCII(value) {
		return value
	}
	return runewidth.Truncate(value, width, "…")
}
