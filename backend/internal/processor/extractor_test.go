package processor

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractCVTextSupportsPlainTextAndPDFTextOperators(t *testing.T) {
	directory := t.TempDir()
	txtPath := filepath.Join(directory, "resume.txt")
	if err := os.WriteFile(txtPath, []byte("Backend Engineer\nGo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	text, err := ExtractCVText(txtPath)
	if err != nil || !strings.Contains(text, "Backend Engineer") {
		t.Fatalf("ExtractCVText(txt) = %q, %v", text, err)
	}

	pdfPath := filepath.Join(directory, "resume.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.7 1 0 obj (Backend Engineer) (Go) endobj"), 0600); err != nil {
		t.Fatal(err)
	}
	text, err = ExtractCVText(pdfPath)
	if err != nil || !strings.Contains(text, "Backend Engineer") || !strings.Contains(text, "Go") {
		t.Fatalf("ExtractCVText(pdf) = %q, %v", text, err)
	}
}

func TestExtractCVTextReadsDOCXDocumentXml(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "resume.docx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`<w:document xmlns:w="urn:word"><w:body><w:p><w:r><w:t>Backend Engineer</w:t></w:r></w:p></w:body></w:document>`))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	text, err := ExtractCVText(path)
	if err != nil || text != "Backend Engineer" {
		t.Fatalf("ExtractCVText(docx) = %q, %v", text, err)
	}
}

func TestExtractCVTextRejectsEmptyOrUnsupportedFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "resume.txt")
	if err := os.WriteFile(path, bytes.Repeat([]byte{' '}, 4), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractCVText(path); err == nil {
		t.Fatal("ExtractCVText() error = nil for empty text")
	}
	if _, err := ExtractCVText(filepath.Join(directory, "resume.doc")); err == nil {
		t.Fatal("ExtractCVText() error = nil for unsupported extension")
	}
}
