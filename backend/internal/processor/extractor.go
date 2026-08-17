package processor

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxExtractedCVBytes = 12 * 1024 * 1024

func ExtractCVText(path string) (string, error) {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".txt":
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return cleanExtractedText(string(content))
	case ".docx":
		return extractDOCXText(path)
	case ".pdf":
		return extractPDFText(path)
	default:
		return "", errors.New("unsupported CV extension")
	}
}

func extractDOCXText(path string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		var builder strings.Builder
		decoder := xml.NewDecoder(io.LimitReader(reader, maxExtractedCVBytes))
		for {
			token, tokenErr := decoder.Token()
			if errors.Is(tokenErr, io.EOF) {
				break
			}
			if tokenErr != nil {
				_ = reader.Close()
				return "", tokenErr
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "t" {
				continue
			}
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				_ = reader.Close()
				return "", err
			}
			builder.WriteString(value)
			builder.WriteByte('\n')
		}
		_ = reader.Close()
		return cleanExtractedText(builder.String())
	}
	return "", errors.New("DOCX document XML is missing")
}

func extractPDFText(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(content) > maxExtractedCVBytes {
		return "", errors.New("CV content is too large")
	}
	var builder strings.Builder
	for index := 0; index < len(content); index++ {
		if content[index] != '(' {
			continue
		}
		index++
		depth := 1
		for index < len(content) && depth > 0 {
			character := content[index]
			if character == '\\' && index+1 < len(content) {
				index++
				switch content[index] {
				case 'n':
					builder.WriteByte('\n')
				case 'r':
					builder.WriteByte('\r')
				case 't':
					builder.WriteByte('\t')
				default:
					builder.WriteByte(content[index])
				}
				index++
				continue
			}
			switch character {
			case '(':
				depth++
				builder.WriteByte(character)
			case ')':
				depth--
				if depth > 0 {
					builder.WriteByte(character)
				}
			default:
				if character == '\n' || character == '\r' || character >= 32 && character <= 126 || character >= 128 {
					builder.WriteByte(character)
				}
			}
			index++
		}
		builder.WriteByte('\n')
	}
	return cleanExtractedText(builder.String())
}

func cleanExtractedText(value string) (string, error) {
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "")
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("CV text is empty or invalid")
	}
	return strings.TrimSpace(value), nil
}
