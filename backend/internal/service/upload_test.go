package service

import "testing"

func TestAcceptedUploadExtensions(t *testing.T) {
	for _, filename := range []string{"resume.pdf", "resume.PDF", "resume.docx", "resume.txt"} {
		if !isAcceptedExtension(filename) {
			t.Errorf("isAcceptedExtension(%q) = false, want true", filename)
		}
	}
}

func TestRejectedUploadExtensions(t *testing.T) {
	for _, filename := range []string{"resume", "resume.exe", "resume.pdf.exe", "resume.doc"} {
		if isAcceptedExtension(filename) {
			t.Errorf("isAcceptedExtension(%q) = true, want false", filename)
		}
	}
}
