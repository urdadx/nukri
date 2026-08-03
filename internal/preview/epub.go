package preview

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"

	epubparser "github.com/mathieu-keller/epub-parser/v2"
	"github.com/mathieu-keller/epub-parser/v2/model"
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
	book, parserErr := parseEPUBBook(reader)
	var metadata model.Metadata
	if parserErr == nil {
		metadata = book.Metadata
	} else {
		metadata, err = parseEPUBMetadata(reader)
		if err != nil {
			return nil, fmt.Errorf("parse EPUB: %v; metadata fallback: %w", parserErr, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	epub := epubMetadata(metadata)
	return &EPUBPreview{Book: epub, Metadata: epubFields(epub)}, nil
}

type fallbackPackage struct {
	UniqueIdentifier string           `xml:"unique-identifier,attr"`
	Metadata         fallbackMetadata `xml:"metadata"`
}

type fallbackMetadata struct {
	Titles       []fallbackText       `xml:"title"`
	Identifiers  []fallbackIdentifier `xml:"identifier"`
	Languages    []string             `xml:"language"`
	Creators     []fallbackText       `xml:"creator"`
	Publishers   []fallbackText       `xml:"publisher"`
	Subjects     []fallbackText       `xml:"subject"`
	Descriptions []fallbackText       `xml:"description"`
	Dates        []string             `xml:"date"`
}

type fallbackText struct {
	Text string `xml:",chardata"`
}

type fallbackIdentifier struct {
	ID   string `xml:"id,attr"`
	Text string `xml:",chardata"`
}

func parseEPUBMetadata(reader *zip.Reader) (model.Metadata, error) {
	var container model.Container
	if err := decodeEPUBXML(reader, "META-INF/container.xml", &container); err != nil {
		return model.Metadata{}, err
	}
	var document fallbackPackage
	if err := decodeEPUBXML(reader, container.Rootfile.Path, &document); err != nil {
		return model.Metadata{}, err
	}
	metadata := model.Metadata{}
	titles := make([]model.Title, 0, len(document.Metadata.Titles))
	for _, value := range document.Metadata.Titles {
		titles = append(titles, model.Title{Title: value.Text})
	}
	metadata.Titles = &titles
	identifiers := make([]model.Identifier, 0, len(document.Metadata.Identifiers))
	for _, value := range document.Metadata.Identifiers {
		identifier := model.Identifier{Id: value.Text}
		identifiers = append(identifiers, identifier)
		if value.ID == document.UniqueIdentifier {
			metadata.MainId = identifier
		}
	}
	if metadata.MainId.Id == "" && len(identifiers) != 0 {
		metadata.MainId = identifiers[0]
	}
	metadata.Identifiers = &identifiers
	metadata.Languages = stringValues(document.Metadata.Languages)
	metadata.Creators = creatorValues(document.Metadata.Creators)
	metadata.Publishers = attributeValues(document.Metadata.Publishers)
	metadata.Subjects = attributeValues(document.Metadata.Subjects)
	metadata.Descriptions = attributeValues(document.Metadata.Descriptions)
	metadata.Dates = stringValues(document.Metadata.Dates)
	return metadata, nil
}

func decodeEPUBXML(reader *zip.Reader, name string, target any) error {
	for _, entry := range reader.File {
		if entry.Name != name {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			return err
		}
		defer file.Close()
		return xml.NewDecoder(io.LimitReader(file, DefaultMaxEPUBEntryBytes+1)).Decode(target)
	}
	return fmt.Errorf("EPUB entry %q does not exist", name)
}

func stringValues(values []string) *[]string {
	result := append([]string(nil), values...)
	return &result
}

func creatorValues(values []fallbackText) *[]model.Creator {
	result := make([]model.Creator, 0, len(values))
	for _, value := range values {
		result = append(result, model.Creator{Name: value.Text})
	}
	return &result
}

func attributeValues(values []fallbackText) *[]model.DefaultAttributes {
	result := make([]model.DefaultAttributes, 0, len(values))
	for _, value := range values {
		result = append(result, model.DefaultAttributes{Text: value.Text})
	}
	return &result
}

func parseEPUBBook(reader *zip.Reader) (book *model.Book, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("parser panic: %v", recovered)
		}
	}()
	return epubparser.OpenBook(reader)
}

func epubMetadata(metadata model.Metadata) EPUB {
	result := EPUB{Identifier: safeText(metadata.MainId.Id)}
	if metadata.Titles != nil {
		for _, value := range *metadata.Titles {
			result.Titles = appendNonempty(result.Titles, value.Title)
		}
	}
	if metadata.Creators != nil {
		for _, value := range *metadata.Creators {
			result.Creators = appendNonempty(result.Creators, value.Name)
		}
	}
	if metadata.Languages != nil {
		for _, value := range *metadata.Languages {
			result.Languages = appendNonempty(result.Languages, value)
		}
	}
	if metadata.Publishers != nil {
		for _, value := range *metadata.Publishers {
			result.Publishers = appendNonempty(result.Publishers, value.Text)
		}
	}
	if metadata.Subjects != nil {
		for _, value := range *metadata.Subjects {
			result.Subjects = appendNonempty(result.Subjects, value.Text)
		}
	}
	if metadata.Descriptions != nil {
		for _, value := range *metadata.Descriptions {
			result.Descriptions = appendNonempty(result.Descriptions, value.Text)
		}
	}
	if metadata.Dates != nil {
		for _, value := range *metadata.Dates {
			result.Dates = appendNonempty(result.Dates, value)
		}
	}
	return result
}

func appendNonempty(values []string, value string) []string {
	value = safeText(value)
	if value != "" {
		return append(values, value)
	}
	return values
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
