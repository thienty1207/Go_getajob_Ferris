package service

import "testing"

func TestNormalizeCrawlerIntervalUsesHoursAndMinutes(t *testing.T) {
	seconds, err := NormalizeCrawlerInterval(6, 30)
	if err != nil {
		t.Fatalf("NormalizeCrawlerInterval() error = %v", err)
	}
	if seconds != 23_400 {
		t.Fatalf("seconds = %d, want 23400", seconds)
	}
}

func TestNormalizeCrawlerIntervalRejectsValuesOutsideOperationalBounds(t *testing.T) {
	for _, test := range []struct {
		name    string
		hours   int
		minutes int
	}{
		{name: "below minimum", hours: 0, minutes: 14},
		{name: "minutes outside hour", hours: 1, minutes: 60},
		{name: "negative hours", hours: -1, minutes: 0},
		{name: "above maximum", hours: 168, minutes: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalizeCrawlerInterval(test.hours, test.minutes); err == nil {
				t.Fatal("NormalizeCrawlerInterval() error = nil, want validation error")
			}
		})
	}
}

func TestCrawlerSettingsFromSecondsSplitsTheStoredValueForTheUI(t *testing.T) {
	settings := CrawlerSettingsFromSeconds(23_400)
	if settings.IntervalHours != 6 || settings.IntervalMinutes != 30 || settings.IntervalSeconds != 23_400 {
		t.Fatalf("settings = %#v, want 6h30m/23400s", settings)
	}
}
