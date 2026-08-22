package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// Upload errors are intentionally coarse at the API boundary so clients do
	// not learn filesystem or parser implementation details.
	ErrInvalidUpload  = errors.New("invalid upload")
	ErrUploadTooLarge = errors.New("upload too large")
)

const (
	cvTemporaryDirectoryName = "gogetsomefoodferris-cv"
	maxDOCXDocumentBytes     = 5 * 1024 * 1024
	// Processing is bounded to two minutes. Fifteen minutes leaves a generous
	// safety margin for shutdown while ensuring a crash cannot retain a raw CV
	// indefinitely on the next application start.
	staleTemporaryCVAge = 15 * time.Minute
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
	extension := strings.ToLower(filepath.Ext(header.Filename))
	if maxBytes <= 0 {
		return "", ErrInvalidUpload
	}

	input, err := header.Open()
	if err != nil {
		return "", ErrInvalidUpload
	}
	defer input.Close()

	directory := cvTemporaryDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	// ExtractCVText dispatches by extension. Preserve only the allowlisted
	// extension (never the user-controlled base filename) so the temporary path
	// remains safe while the parser can select PDF, DOCX, or plain text.
	temporary, err := os.CreateTemp(directory, "gogetsomefood-cv-*"+extension)
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	keep := false
	defer func() {
		if !keep {
			if cleanupErr := os.Remove(path); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				// Do not log a path derived from sensitive temporary CV handling.
				slog.Error("temporary CV cleanup failed", "error_code", "temporary_cv_cleanup_failed")
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
	if bytesCopied == 0 {
		_ = temporary.Close()
		return "", ErrInvalidUpload
	}
	if err := temporary.Close(); err != nil {
		return "", ErrInvalidUpload
	}

	if err := validateFileSignature(path, extension); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

func cvTemporaryDirectory() string {
	return filepath.Join(os.TempDir(), cvTemporaryDirectoryName)
}

// cleanupStaleTemporaryCVs is the crash-recovery half of the raw-CV retention
// contract. It never follows directories or touches files outside the private
// CV directory, and it leaves recent files alone in case another process is
// still completing a bounded scan.
func cleanupStaleTemporaryCVs(directory string, now time.Time) error {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var cleanupErr error
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "gogetsomefood-cv-") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			cleanupErr = errors.Join(cleanupErr, infoErr)
			continue
		}
		if !info.Mode().IsRegular() || now.Sub(info.ModTime()) < staleTemporaryCVAge {
			continue
		}
		if removeErr := os.Remove(filepath.Join(directory, entry.Name())); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, removeErr)
		}
	}
	return cleanupErr
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
		archive, err := zip.OpenReader(path)
		if err != nil {
			return ErrInvalidUpload
		}
		defer archive.Close()
		for _, entry := range archive.File {
			if entry.Name == "word/document.xml" && !entry.FileInfo().IsDir() && entry.UncompressedSize64 > 0 && entry.UncompressedSize64 <= maxDOCXDocumentBytes {
				return nil
			}
		}
		return ErrInvalidUpload
	}
	return ErrInvalidUpload
}
