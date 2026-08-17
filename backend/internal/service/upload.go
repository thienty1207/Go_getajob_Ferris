package service

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

var (
	// Upload errors are intentionally coarse at the API boundary so clients do
	// not learn filesystem or parser implementation details.
	ErrInvalidUpload  = errors.New("invalid upload")
	ErrUploadTooLarge = errors.New("upload too large")
)

// isAcceptedExtension is an early user-experience check only. The file
// signature check below remains authoritative because filenames are untrusted.
func isAcceptedExtension(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf", ".docx", ".txt":
		return true
	default:
		return false
	}
}

// saveTemporaryCV copies the upload to an OS temporary file for the future
// parser and guarantees cleanup unless the caller receives a valid path. Raw
// CV content is never inserted into PostgreSQL by this function.
func saveTemporaryCV(header *multipart.FileHeader, maxBytes int64) (string, error) {
	if header == nil || !isAcceptedExtension(header.Filename) {
		return "", ErrInvalidUpload
	}
	if maxBytes <= 0 {
		return "", ErrInvalidUpload
	}

	input, err := header.Open()
	if err != nil {
		return "", ErrInvalidUpload
	}
	defer input.Close()

	temporary, err := os.CreateTemp("", "gogetsomefood-cv-*")
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	keep := false
	defer func() {
		if !keep {
			if cleanupErr := os.Remove(path); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				slog.Error("temporary CV cleanup failed", "err", cleanupErr)
			}
		}
	}()

	bytesCopied, err := io.Copy(temporary, io.LimitReader(input, maxBytes+1))
	if err != nil {
		_ = temporary.Close()
		return "", ErrInvalidUpload
	}
	if bytesCopied > maxBytes {
		_ = temporary.Close()
		return "", ErrUploadTooLarge
	}
	if err := temporary.Close(); err != nil {
		return "", ErrInvalidUpload
	}

	extension := strings.ToLower(filepath.Ext(header.Filename))
	if err := validateFileSignature(path, extension); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

// validateFileSignature checks the bytes after copying, preventing a renamed
// executable or HTML file from being accepted as a CV document.
func validateFileSignature(path, extension string) error {
	if extension == ".txt" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return ErrInvalidUpload
	}
	defer file.Close()

	signature := make([]byte, 5)
	bytesRead, err := io.ReadFull(file, signature)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return ErrInvalidUpload
	}
	if extension == ".pdf" {
		if bytesRead < 5 || !bytes.Equal(signature, []byte("%PDF-")) {
			return ErrInvalidUpload
		}
		return nil
	}

	if extension == ".docx" {
		if bytesRead < 2 || signature[0] != 'P' || signature[1] != 'K' {
			return ErrInvalidUpload
		}
		return nil
	}
	return ErrInvalidUpload
}
