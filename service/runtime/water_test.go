package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainwater "empirebus-tests/service/domains/water"
)

type stubWaterController struct {
	mu       sync.Mutex
	state    domainwater.State
	commands []string
	holds    []time.Duration
	err      error
	block    chan struct{}
}

func (s *stubWaterController) OpenGreyWaterValve(_ context.Context, hold time.Duration) error {
	return s.record("open", hold)
}

func (s *stubWaterController) CloseGreyWaterValve(_ context.Context, hold time.Duration) error {
	return s.record("close", hold)
}

func (s *stubWaterController) WaterState() domainwater.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *stubWaterController) record(command string, hold time.Duration) error {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, command)
	s.holds = append(s.holds, hold)
	if s.err != nil {
		return s.err
	}
	s.state.ValveKnown = true
	s.state.ValveMoving = false
	return nil
}

func TestOpenGreyWaterValveUsesFiveSecondHold(t *testing.T) {
	t.Parallel()

	controller := &stubWaterController{}
	app := &App{water: controller}
	if err := app.OpenGreyWaterValve(context.Background()); err != nil {
		t.Fatalf("OpenGreyWaterValve returned error: %v", err)
	}
	if got, want := controller.commands, []string{"open"}; !equalStrings(got, want) {
		t.Fatalf("unexpected commands: got=%v want=%v", got, want)
	}
	if len(controller.holds) != 1 || controller.holds[0] != 5*time.Second {
		t.Fatalf("expected one 5s hold, got %v", controller.holds)
	}
	if state := app.WaterState(); state.CommandInProgress {
		t.Fatalf("expected command flag cleared")
	}
}

func TestWaterCommandRejectsBusy(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	controller := &stubWaterController{block: block}
	app := &App{water: controller}
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.OpenGreyWaterValve(context.Background())
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if app.WaterState().CommandInProgress {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for water command to begin")
		}
		time.Sleep(10 * time.Millisecond)
	}

	err := app.CloseGreyWaterValve(context.Background())
	if !errors.Is(err, ErrWaterCommandInProgress) {
		t.Fatalf("expected ErrWaterCommandInProgress, got %v", err)
	}
	close(block)
	if err := <-errCh; err != nil {
		t.Fatalf("expected first command to finish cleanly, got %v", err)
	}
}

func TestWaterCommandRecordsError(t *testing.T) {
	t.Parallel()

	controller := &stubWaterController{err: errors.New("valve failed")}
	app := &App{water: controller}
	err := app.CloseGreyWaterValve(context.Background())
	if err == nil || err.Error() != "valve failed" {
		t.Fatalf("expected valve failure, got %v", err)
	}
	if state := app.WaterState(); state.LastCommandError != "valve failed" {
		t.Fatalf("expected command error, got %q", state.LastCommandError)
	}
}
