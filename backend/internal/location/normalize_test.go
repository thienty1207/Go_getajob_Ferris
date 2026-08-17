package location

import "testing"

func TestNormalizeLocation(t *testing.T) {
	tests := map[string]string{
		"TP.HCM":             "tp hcm",
		"Ho Chi Minh City":   "ho chi minh city",
		"Q.1, TP.HCM":        "q 1 tp hcm",
		"  Hà Nội  ":         "ha noi",
		"Đà Nẵng / Việt Nam": "da nang viet nam",
	}
	for input, want := range tests {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeLocationRejectsEmptyAndOversizedInput(t *testing.T) {
	if got := Normalize("   "); got != "" {
		t.Fatalf("Normalize(blank) = %q, want empty", got)
	}
	if got := Normalize("x" + string(make([]byte, 513))); got != "" {
		t.Fatalf("Normalize(oversized) = %q, want empty", got)
	}
}
