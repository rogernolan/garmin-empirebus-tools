package runtime

import (
	"testing"
	"time"

	domainlocation "empirebus-tests/service/domains/location"
)

func TestRecentLocationFixesKeepsWindow(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	fixes := []domainlocation.Fix{
		{Latitude: 45.0, Longitude: 10.0, UpdatedAt: now.Add(-20 * time.Minute)},
		{Latitude: 45.0, Longitude: 10.001, UpdatedAt: now.Add(-10 * time.Minute)},
		{Latitude: 45.0, Longitude: 10.002, UpdatedAt: now},
	}
	got := recentLocationFixes(fixes, now, 15*time.Minute)
	if len(got) != 2 {
		t.Fatalf("got %d fixes", len(got))
	}
	if got[0].Longitude != 10.001 {
		t.Fatalf("old fix was not trimmed: %#v", got[0])
	}
}

func TestCumulativeMovementMeters(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	fixes := []domainlocation.Fix{
		{Latitude: 45.0, Longitude: 10.0, UpdatedAt: now.Add(-10 * time.Minute)},
		{Latitude: 45.0, Longitude: 10.001, UpdatedAt: now.Add(-5 * time.Minute)},
		{Latitude: 45.0, Longitude: 10.002, UpdatedAt: now},
	}
	got := cumulativeMovementMeters(fixes)
	if got < 150 || got > 160 {
		t.Fatalf("got movement %.2fm", got)
	}
}
