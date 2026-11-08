package preview

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

func (s *Service) renderEPUB(ctx context.Context, path string) (*EPUBPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open EPUB: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect EPUB: %w", err)
	}
	if info.Size() > s.maxEPUBBytes {
		return nil, ErrOutputTooLarge
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, fmt.Errorf("open EPUB container: %w", err)
	}
	if len(reader.File) > s.maxArchiveEntries {
		return nil, ErrOutputTooLarge
	}
	var totalBytes uint64
	for _, entry := range reader.File {
		if entry.UncompressedSize64 > s.maxEPUBEntryBytes || totalBytes > s.maxEPUBTotalBytes-entry.UncompressedSize64 {
			return nil, ErrOutputTooLarge
		}
		totalBytes += entry.UncompressedSize64
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	epub, err := parseEPUBMetadata(reader)
	if err != nil {
		return nil, fmt.Errorf("parse EPUB: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &EPUBPreview{Book: epub, Metadata: epubFields(epub)}, nil
}

// epubContainer is META-INF/container.xml, which points at the package document.
type epubContainer struct {
	Rootfile struct {
		Path string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

// epubPackage holds the Dublin Core metadata of the package document (OPF).
type epubPackage struct {
	UniqueIdentifier string `xml:"unique-identifier,attr"`
	Metadata         struct {
		Titles      []string `xml:"title"`
		Identifiers []struct {
			ID   string `xml:"id,attr"`
			Text string `xml:",chardata"`
		} `xml:"identifier"`
		Languages    []string `xml:"language"`
		Creators     []string `xml:"creator"`
		Publishers   []string `xml:"publisher"`
		Subjects     []string `xml:"subject"`
		Descriptions []string `xml:"description"`
		Dates        []string `xml:"date"`
	} `xml:"metadata"`
}

func parseEPUBMetadata(reader *zip.Reader) (EPUB, error) {
	var container epubContainer
	if err := decodeEPUBXML(reader, "META-INF/container.xml", &container); err != nil {
		return EPUB{}, err
	}
	document, err := decodeEPUBPackage(reader, container.Rootfile.Path)
	if err != nil {
		return EPUB{}, err
	}
	metadata := document.Metadata
	result := EPUB{
		Titles:       nonemptyValues(metadata.Titles),
		Creators:     nonemptyValues(metadata.Creators),
		Languages:    nonemptyValues(metadata.Languages),
		Publishers:   nonemptyValues(metadata.Publishers),
		Subjects:     nonemptyValues(metadata.Subjects),
		Descriptions: nonemptyValues(metadata.Descriptions),
		Dates:        nonemptyValues(metadata.Dates),
	}
	// Prefer the identifier the package names as unique, else the first one.
	for i, identifier := range metadata.Identifiers {
		if i == 0 || identifier.ID == document.UniqueIdentifier {
			result.Identifier = safeText(strings.TrimSpace(identifier.Text))
		}
		if identifier.ID == document.UniqueIdentifier {
			break
		}
	}
	return result, nil
}

/*
decodeEPUBPackage reads the package document's unique-identifier attribute and
its metadata, then stops. The manifest and spine that follow list every file in
the book, and decoding them was most of the cost of a large EPUB's preview.
*/
func decodeEPUBPackage(reader *zip.Reader, name string) (epubPackage, error) {
	var document epubPackage
	file, err := openEPUBEntry(reader, name)
	if err != nil {
		return document, err
	}
	defer file.Close()
	decoder := xml.NewDecoder(io.LimitReader(file, DefaultMaxEPUBEntryBytes+1))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return document, nil // no metadata element: an empty preview
		}
		if err != nil {
			return document, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "package":
			for _, attribute := range start.Attr {
				if attribute.Name.Local == "unique-identifier" {
					document.UniqueIdentifier = attribute.Value
				}
			}
		case "metadata":
			return document, decoder.DecodeElement(&document.Metadata, &start)
		}
	}
}

func openEPUBEntry(reader *zip.Reader, name string) (io.ReadCloser, error) {
	for _, entry := range reader.File {
		if entry.Name == name {
			return entry.Open()
		}
	}
	return nil, fmt.Errorf("EPUB entry %q does not exist", name)
}

func decodeEPUBXML(reader *zip.Reader, name string, target any) error {
	file, err := openEPUBEntry(reader, name)
	if err != nil {
		return err
	}
	defer file.Close()
	return xml.NewDecoder(io.LimitReader(file, DefaultMaxEPUBEntryBytes+1)).Decode(target)
}

func nonemptyValues(values []string) []string {
	var result []string
	for _, value := range values {
		if value = safeText(strings.TrimSpace(value)); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func epubFields(value EPUB) []Field {
	fields := make([]Field, 0, 4)
	if value.Identifier != "" {
		fields = append(fields, Field{Name: "Identifier", Value: value.Identifier})
	}
	for _, item := range value.Titles {
		fields = append(fields, Field{Name: "Title", Value: item})
	}
	for _, item := range value.Creators {
		fields = append(fields, Field{Name: "Creator", Value: item})
	}
	for _, item := range value.Languages {
		fields = append(fields, Field{Name: "Language", Value: item})
	}
	return fields
}
