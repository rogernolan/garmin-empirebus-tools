package tzfresolver

import (
	"context"
	"testing"
)

type fakeFinder struct {
	gotLng float64
	gotLat float64
	name   string
}

func (f *fakeFinder) GetTimezoneName(lng float64, lat float64) string {
	f.gotLng = lng
	f.gotLat = lat
	return f.name
}

func TestResolverUsesLongitudeLatitudeOrder(t *testing.T) {
	finder := &fakeFinder{name: "Europe/Rome"}
	resolver := NewWithFinder(finder)
	timezoneName, err := resolver.Timezone(context.Background(), 46.23759, 9.130995)
	if err != nil {
		t.Fatal(err)
	}
	if timezoneName != "Europe/Rome" {
		t.Fatalf("got timezone %q", timezoneName)
	}
	if finder.gotLng != 9.130995 {
		t.Fatalf("got longitude %f", finder.gotLng)
	}
	if finder.gotLat != 46.23759 {
		t.Fatalf("got latitude %f", finder.gotLat)
	}
}

func TestResolverRejectsEmptyTimezone(t *testing.T) {
	resolver := NewWithFinder(&fakeFinder{})
	if _, err := resolver.Timezone(context.Background(), 0, 0); err == nil {
		t.Fatal("expected empty timezone error")
	}
}

func TestResolverRejectsInvalidTimezone(t *testing.T) {
	resolver := NewWithFinder(&fakeFinder{name: "not-a-timezone"})
	if _, err := resolver.Timezone(context.Background(), 0, 0); err == nil {
		t.Fatal("expected invalid timezone error")
	}
}
