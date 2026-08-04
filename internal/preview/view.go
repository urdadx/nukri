package preview

import (
	"fmt"
	"strconv"
	"strings"
)

type ScrollMode int

const (
	NoScroll ScrollMode = iota
	VerticalScroll
	HorizontalScroll
	BothScroll
)

type ViewOptions struct {
	Width        int
	ColumnOffset int
}

// View is the presentation boundary consumed by a preview-pane UI. Visual is
// encoded image data; terminal protocol rendering remains the UI's concern.
type View struct {
	Title  string
	Detail string
	Lines  []string
	Visual *Image
	Footer string
	Scroll ScrollMode
}

func BuildView(value Preview, options ViewOptions) (View, error) {
	width := options.Width
	if width <= 0 {
		width = DefaultCSVTableWidth
	}
	switch value := value.(type) {
	case *PDFPreview:
		return metadataVisualView("PDF", "First page", value.Metadata, &value.Page), nil
	case *SVGPreview:
		return View{Title: "SVG", Detail: imageDimensions(value.Image), Visual: &value.Image}, nil
	case *OfficePreview:
		return metadataVisualView(value.Format.DetailLabel(), "First page", value.Metadata, &value.Page), nil
	case *EbookPreview:
		return metadataVisualView(value.Format.DetailLabel(), "First page", value.Metadata, &value.Page), nil
	case *MarkdownPreview:
		return View{Title: "Markdown", Lines: strings.Split(value.Text, "\n"), Scroll: VerticalScroll}, nil
	case *DirectoryPreview:
		return directoryView(value, width, options.ColumnOffset), nil
	case *ArchivePreview:
		return archiveView(value, width, options.ColumnOffset), nil
	case *EPUBPreview:
		title := "EPUB"
		if len(value.Book.Titles) != 0 {
			title = value.Book.Titles[0]
		}
		return View{Title: title, Detail: "EPUB ebook", Lines: fieldLines(value.Metadata), Scroll: VerticalScroll}, nil
	case *FontPreview:
		return metadataVisualView(value.Font.Family, value.Font.Subfamily, value.Metadata, &value.Specimen), nil
	case *AudioPreview:
		title := value.Audio.Title
		if title == "" {
			title = "Audio"
		}
		detail := value.Audio.Artist
		if detail == "" {
			detail = value.Audio.Codec
		}
		return metadataVisualView(title, detail, value.Metadata, &value.Visual), nil
	case *VideoPreview:
		title := value.Video.Title
		if title == "" {
			title = "Video"
		}
		detail := value.Video.Codec
		if value.Video.Width != 0 && value.Video.Height != 0 {
			detail = joinDetail(detail, fmt.Sprintf("%dx%d", value.Video.Width, value.Video.Height))
		}
		return metadataVisualView(title, detail, value.Metadata, &value.Frame), nil
	case *CSVPreview:
		return csvView(value, width, options.ColumnOffset), nil
	case nil:
		return View{}, ErrUnsupported
	default:
		return View{}, fmt.Errorf("%w: %T", ErrUnsupported, value)
	}
}

func metadataVisualView(title, detail string, fields []Field, visual *Image) View {
	return View{
		Title: title, Detail: detail, Lines: fieldLines(fields), Visual: visual,
		Scroll: VerticalScroll,
	}
}

func fieldLines(fields []Field) []string {
	width := 0
	for _, field := range fields {
		width = max(width, len([]rune(field.Name)))
	}
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, fmt.Sprintf("%-*s  %s", width, field.Name, field.Value))
	}
	return lines
}

func directoryView(value *DirectoryPreview, width, offset int) View {
	rows := make([][]string, 0, len(value.Entries))
	for _, entry := range value.Entries {
		size := strconv.FormatInt(entry.Size, 10)
		if entry.Directory {
			size = "-"
		}
		rows = append(rows, []string{entry.Mode, size, entry.Modified, entry.Name})
	}
	lines, footer, horizontal := tableLines(
		[]string{"Mode", "Size", "Modified", "Name"}, rows, value.TotalItems, width, offset,
	)
	detail := fmt.Sprintf("%d items, %d folders, %d files", value.TotalItems, value.FolderCount, value.FileCount)
	return View{Title: "Directory", Detail: detail, Lines: lines, Footer: footer, Scroll: tableScroll(horizontal)}
}

func archiveView(value *ArchivePreview, width, offset int) View {
	rows := make([][]string, 0, len(value.Archive.Entries))
	for _, entry := range value.Archive.Entries {
		rows = append(rows, []string{
			entry.Attributes, strconv.FormatInt(entry.Size, 10), strconv.FormatInt(entry.PackedSize, 10), entry.Modified, entry.Path,
		})
	}
	lines, footer, horizontal := tableLines(
		[]string{"Attributes", "Size", "Packed", "Modified", "Path"}, rows, len(value.Archive.Entries), width, offset,
	)
	if value.Archive.Truncated {
		footer = joinFooter(footer, "listing truncated")
	}
	return View{Title: "Archive", Detail: fmt.Sprintf("%d entries", len(value.Archive.Entries)), Lines: lines, Footer: footer, Scroll: tableScroll(horizontal)}
}

func csvView(value *CSVPreview, width, offset int) View {
	table := RenderCSVTable(value, CSVTableOptions{Width: width, ColumnOffset: offset})
	lines := strings.Split(table, "\n")
	footer := ""
	if len(lines) != 0 {
		footer = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}
	title := "CSV"
	if value.Metadata.Delimiter == '\t' {
		title = "TSV"
	}
	horizontal := offset > 0 || value.Metadata.ColumnCount > visibleCSVColumnCount(value, width, offset)
	detail := fmt.Sprintf("%d rows x %d columns", value.Metadata.RowCount, value.Metadata.ColumnCount)
	return View{Title: title, Detail: detail, Lines: lines, Footer: footer, Scroll: tableScroll(horizontal)}
}

func tableLines(headers []string, rows [][]string, totalRows, width, offset int) ([]string, string, bool) {
	allRows := make([][]string, 0, len(rows)+1)
	allRows = append(allRows, headers)
	allRows = append(allRows, rows...)
	columnCount := maxColumns(allRows)
	value := &CSVPreview{
		Rows:          allRows,
		Metadata:      CSVMetadata{RowCount: totalRows, ColumnCount: columnCount, Delimiter: ','},
		RowsTruncated: totalRows > len(rows),
	}
	table := RenderCSVTable(value, CSVTableOptions{Width: width, ColumnOffset: offset})
	lines := strings.Split(table, "\n")
	if len(lines) != 0 {
		lines = lines[:len(lines)-1]
	}
	visibleColumns := visibleCSVColumnCount(value, width, offset)
	firstColumn := min(max(offset, 0), max(columnCount-1, 0)) + 1
	lastColumn := min(firstColumn+visibleColumns-1, columnCount)
	footer := fmt.Sprintf("Columns %d-%d of %d | Showing %d of %d items", firstColumn, lastColumn, columnCount, len(rows), totalRows)
	horizontal := offset > 0 || columnCount > visibleColumns
	return lines, footer, horizontal
}

func visibleCSVColumnCount(value *CSVPreview, width, offset int) int {
	columnWidths := csvColumnWidths(value.Rows, value.Metadata.ColumnCount, DefaultCSVColumnWidth)
	columns, _ := visibleCSVColumns(columnWidths, min(max(offset, 0), max(len(columnWidths)-1, 0)), max(width, minimumCSVTerminalWidth))
	return len(columns)
}

func tableScroll(horizontal bool) ScrollMode {
	if horizontal {
		return BothScroll
	}
	return VerticalScroll
}

func imageDimensions(image Image) string {
	if image.Width == 0 || image.Height == 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", image.Width, image.Height)
}

func joinDetail(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + " | " + right
}

func joinFooter(left, right string) string {
	return joinDetail(left, right)
}
