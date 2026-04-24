package runtime

import "testing"

func TestDefaultLightsStateIsUnknownAndIdle(t *testing.T) {
	app := App{}
	state := app.LightsState()
	if state.ExternalKnown {
		t.Fatalf("expected external state to start unknown")
	}
	if state.ExternalOn {
		t.Fatalf("expected external_on zero value to be false")
	}
	if state.FlashInProgress {
		t.Fatalf("expected flash to start idle")
	}
	if state.LastCommandError != "" {
		t.Fatalf("expected no last command error, got %q", state.LastCommandError)
	}
	if state.LastUpdatedAt != nil {
		t.Fatalf("expected last updated time to start unset")
	}
}
