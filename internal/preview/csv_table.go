package preview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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

// Table colors are ANSI palette indexes, so they follow the terminal theme.
var (
	csvBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	csvHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// drawCSVTable draws rows (the first one as the header) restricted to columns,
// each cell fitted to its width in widths.
func drawCSVTable(rows [][]string, columns, widths []int) string {
	numeric := numericColumns(rows, columns)
	cells := make([][]string, len(rows))
	for rowIndex, row := range rows {
		cells[rowIndex] = make([]string, len(columns))
		for index, column := range columns {
			if column < len(row) {
				cells[rowIndex][index] = runewidth.Truncate(row[column], widths[index], "…")
			}
		}
	}
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(csvBorderStyle).
		BorderRow(false).
		Headers(cells[0]...).
		Rows(cells[1:]...).
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
