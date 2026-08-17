package model

import "testing"

func TestScanStatusValidityAndProcessingMapping(t *testing.T) {
	for _, status := range []ScanStatus{StatusReceived, StatusParsing, StatusMatching, StatusCompleted, StatusFailed} {
		if !status.IsValid() {
			t.Errorf("%q IsValid() = false, want true", status)
		}
	}
	for _, status := range []ScanStatus{StatusReceived, StatusParsing, StatusMatching} {
		if !status.IsProcessing() {
			t.Errorf("%q IsProcessing() = false, want true", status)
		}
	}
	for _, status := range []ScanStatus{StatusCompleted, StatusFailed, ScanStatus("UNKNOWN")} {
		if status.IsProcessing() {
			t.Errorf("%q IsProcessing() = true, want false", status)
		}
	}
	if ScanStatus("UNKNOWN").IsValid() {
		t.Fatal("unknown status IsValid() = true, want false")
	}
}
