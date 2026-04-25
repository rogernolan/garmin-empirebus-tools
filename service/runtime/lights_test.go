package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainlights "empirebus-tests/service/domains/lights"
)

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

func TestMemoryLightsStateTracksExteriorOnOff(t *testing.T) {
	at := time.Unix(1710000000, 0).UTC()
	state := recordExteriorSignal(domainlights.State{}, true, at)
	if !state.ExternalKnown {
		t.Fatalf("expected exterior state to become known")
	}
	if !state.ExternalOn {
		t.Fatalf("expected exterior lights to be on")
	}
	if state.LastUpdatedAt == nil || !state.LastUpdatedAt.Equal(at) {
		t.Fatalf("expected last update timestamp to be recorded")
	}

	state = recordExteriorSignal(state, false, at.Add(time.Second))
	if !state.ExternalKnown {
		t.Fatalf("expected exterior state to remain known")
	}
	if state.ExternalOn {
		t.Fatalf("expected exterior lights to be off after off signal")
	}
}

type stubLightsController struct {
	mu         sync.Mutex
	state      domainlights.State
	commands   []string
	errOnOnAt  int
	errOnOffAt int
	ensureOnN  int
	ensureOffN int
}

func (s *stubLightsController) EnsureExteriorOn(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureOnN++
	if s.errOnOnAt > 0 && s.ensureOnN == s.errOnOnAt {
		return errors.New("ensure exterior on failed")
	}
	s.commands = append(s.commands, "on")
	s.state.ExternalKnown = true
	s.state.ExternalOn = true
	return nil
}

func (s *stubLightsController) EnsureExteriorOff(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureOffN++
	if s.errOnOffAt > 0 && s.ensureOffN == s.errOnOffAt {
		return errors.New("ensure exterior off failed")
	}
	s.commands = append(s.commands, "off")
	s.state.ExternalKnown = true
	s.state.ExternalOn = false
	return nil
}

func (s *stubLightsController) LightsState() domainlights.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *stubLightsController) commandLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

func (s *stubLightsController) ensureCounts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureOnN, s.ensureOffN
}

func TestFlashExternalRestoresOnState(t *testing.T) {
	t.Parallel()

	controller := &stubLightsController{
		state: domainlights.State{
			ExternalKnown: true,
			ExternalOn:    true,
		},
	}
	var sleeps []time.Duration
	app := &App{
		lights: controller,
		sleep: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
	}

	if err := app.FlashExteriorLights(context.Background(), 2); err != nil {
		t.Fatalf("FlashExteriorLights returned error: %v", err)
	}

	if got, want := controller.commandLog(), []string{"on", "off", "on", "off", "on"}; !equalStrings(got, want) {
		t.Fatalf("unexpected commands: got=%v want=%v", got, want)
	}
	if len(sleeps) != 4 {
		t.Fatalf("expected 4 sleep calls, got %d", len(sleeps))
	}
	for i, got := range sleeps {
		if got != 500*time.Millisecond {
			t.Fatalf("sleep %d: expected 500ms, got %s", i, got)
		}
	}
	state := app.LightsState()
	if !state.ExternalKnown || !state.ExternalOn {
		t.Fatalf("expected runtime lights state restored to known on, got known=%t on=%t", state.ExternalKnown, state.ExternalOn)
	}
	if state.FlashInProgress {
		t.Fatalf("expected flash flag cleared after completion")
	}
	if state.LastCommandError != "" {
		t.Fatalf("expected last command error cleared, got %q", state.LastCommandError)
	}
}

func TestFlashExternalRejectsBusy(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	controller := &stubLightsController{
		state: domainlights.State{
			ExternalKnown: true,
			ExternalOn:    false,
		},
	}
	app := &App{
		lights: controller,
		sleep: func(time.Duration) {
			<-block
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.FlashExteriorLights(context.Background(), 1)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if app.LightsState().FlashInProgress {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for flash to begin")
		}
		time.Sleep(10 * time.Millisecond)
	}

	err := app.FlashExteriorLights(context.Background(), 1)
	if !errors.Is(err, ErrLightsFlashInProgress) {
		t.Fatalf("expected ErrLightsFlashInProgress, got %v", err)
	}
	if state := app.LightsState(); state.LastCommandError != ErrLightsFlashInProgress.Error() {
		t.Fatalf("expected busy error to be recorded, got %q", state.LastCommandError)
	}

	close(block)
	if err := <-errCh; err != nil {
		t.Fatalf("expected first flash to finish cleanly, got %v", err)
	}
}

func TestFlashExternalRejectsInvalidCount(t *testing.T) {
	t.Parallel()

	app := &App{}
	for _, count := range []int{0, 6} {
		err := app.FlashExteriorLights(context.Background(), count)
		if !errors.Is(err, ErrInvalidFlashCount) {
			t.Fatalf("count=%d: expected ErrInvalidFlashCount, got %v", count, err)
		}
		if state := app.LightsState(); state.LastCommandError != ErrInvalidFlashCount.Error() {
			t.Fatalf("count=%d: expected invalid-count error to be recorded, got %q", count, state.LastCommandError)
		}
	}
}

func TestFlashExternalRecordsLastCommandErrorAndClearsBusy(t *testing.T) {
	t.Parallel()

	controller := &stubLightsController{
		state: domainlights.State{
			ExternalKnown: true,
			ExternalOn:    false,
		},
		errOnOffAt: 1,
	}
	app := &App{
		lights: controller,
		sleep:  func(time.Duration) {},
	}

	err := app.FlashExteriorLights(context.Background(), 1)
	if err == nil || err.Error() != "ensure exterior off failed" {
		t.Fatalf("expected ensure exterior off failure, got %v", err)
	}

	state := app.LightsState()
	if state.FlashInProgress {
		t.Fatalf("expected flash flag cleared after error")
	}
	if state.LastCommandError != "ensure exterior off failed" {
		t.Fatalf("expected last command error to be recorded, got %q", state.LastCommandError)
	}
}

func TestFlashExternalAttemptsRestoreAfterLoopFailure(t *testing.T) {
	t.Parallel()

	controller := &stubLightsController{
		state: domainlights.State{
			ExternalKnown: true,
			ExternalOn:    true,
		},
		errOnOffAt: 1,
		errOnOnAt:  2,
	}
	app := &App{
		lights: controller,
		sleep:  func(time.Duration) {},
	}

	err := app.FlashExteriorLights(context.Background(), 1)
	if err == nil || err.Error() != "ensure exterior off failed" {
		t.Fatalf("expected original loop failure to be returned, got %v", err)
	}

	ensureOnN, ensureOffN := controller.ensureCounts()
	if ensureOnN != 2 || ensureOffN != 1 {
		t.Fatalf("expected restore attempt after loop failure, got ensureOn=%d ensureOff=%d", ensureOnN, ensureOffN)
	}
	if state := app.LightsState(); state.FlashInProgress {
		t.Fatalf("expected flash flag cleared after failed restore")
	}
	if state := app.LightsState(); state.LastCommandError != "ensure exterior off failed" {
		t.Fatalf("expected original command error to be preserved, got %q", state.LastCommandError)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
