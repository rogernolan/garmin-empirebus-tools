package runtime

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"empirebus-tests/service/api/events"
	"empirebus-tests/service/config"
	domainheating "empirebus-tests/service/domains/heating"
)

type fakeHeatingController struct {
	ensureOnCalls  int
	ensureOffCalls int
	setTargetCalls []float64
	ensureOnErr    error
	ensureOffErr   error
	setTargetErr   error
}

func (f *fakeHeatingController) EnsureOn(context.Context) error {
	f.ensureOnCalls++
	return f.ensureOnErr
}

func (f *fakeHeatingController) EnsureOff(context.Context) error {
	f.ensureOffCalls++
	return f.ensureOffErr
}

func (f *fakeHeatingController) SetTargetTemperature(_ context.Context, celsius float64) error {
	f.setTargetCalls = append(f.setTargetCalls, celsius)
	return f.setTargetErr
}

func (f *fakeHeatingController) CurrentState() HeatingStateView {
	return domainheating.State{}
}

func (f *fakeHeatingController) Health() domainheating.AdapterHealth {
	return domainheating.AdapterHealth{}
}

func TestSetHeatingModeManualPersistsAndApplies(t *testing.T) {
	t.Parallel()
	adapter := &fakeHeatingController{}
	app := &App{
		adapter:          adapter,
		broker:           events.NewBroker(1),
		logger:           log.New(io.Discard, "", 0),
		runtimeStatePath: filepath.Join(t.TempDir(), "runtime.yaml"),
		schedulerWake:    make(chan struct{}, 1),
	}
	state, err := app.SetHeatingModeManual(context.Background(), 19.0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != config.HeatingModeManual {
		t.Fatalf("got mode %q", state.Mode)
	}
	if adapter.ensureOnCalls != 1 {
		t.Fatalf("got ensureOnCalls=%d", adapter.ensureOnCalls)
	}
	if len(adapter.setTargetCalls) != 1 || adapter.setTargetCalls[0] != 19.0 {
		t.Fatalf("unexpected set target calls %#v", adapter.setTargetCalls)
	}
}

func TestTimezoneFromLocaltimeTarget(t *testing.T) {
	t.Parallel()
	if got := timezoneFromLocaltimeTarget("../usr/share/zoneinfo/Europe/Rome"); got != "Europe/Rome" {
		t.Fatalf("got timezone %q", got)
	}
}

func TestSetHeatingModeManualDoesNotPersistWhenApplyFails(t *testing.T) {
	t.Parallel()
	adapter := &fakeHeatingController{setTargetErr: errors.New("set target failed")}
	runtimePath := filepath.Join(t.TempDir(), "runtime.yaml")
	app := &App{
		adapter:          adapter,
		broker:           events.NewBroker(1),
		logger:           log.New(io.Discard, "", 0),
		runtimeStatePath: runtimePath,
		modeState:        config.DefaultHeatingRuntimeState(),
		schedulerWake:    make(chan struct{}, 1),
	}

	if _, err := app.SetHeatingModeManual(context.Background(), 19.0); err == nil {
		t.Fatal("expected apply error")
	}
	if app.HeatingMode().Mode != config.HeatingModeSchedule {
		t.Fatalf("mode persisted in memory after apply failure: %q", app.HeatingMode().Mode)
	}
	if _, err := os.Stat(runtimePath); !os.IsNotExist(err) {
		t.Fatalf("runtime state was persisted after apply failure: %v", err)
	}
}

func TestCollapseExpiredBoostRestoresResumeMode(t *testing.T) {
	t.Parallel()
	manual := 18.5
	state := config.HeatingRuntimeState{
		Mode: config.HeatingModeBoost,
		Boost: &config.HeatingBoostState{
			TargetCelsius:             22.0,
			ExpiresAt:                 time.Now().UTC().Add(-time.Minute),
			ResumeMode:                config.HeatingModeManual,
			ResumeManualTargetCelsius: &manual,
		},
	}
	expired, collapsed := collapseExpiredBoost(state, time.Now().UTC())
	if !expired {
		t.Fatal("expected boost to be expired")
	}
	if collapsed.Mode != config.HeatingModeManual {
		t.Fatalf("got mode %q", collapsed.Mode)
	}
	if collapsed.ManualTargetCelsius == nil || *collapsed.ManualTargetCelsius != manual {
		t.Fatalf("unexpected manual target %#v", collapsed.ManualTargetCelsius)
	}
}

func TestCancelHeatingModeBoostRestoresResumeMode(t *testing.T) {
	t.Parallel()
	adapter := &fakeHeatingController{}
	app := &App{
		adapter:          adapter,
		broker:           events.NewBroker(1),
		logger:           log.New(io.Discard, "", 0),
		runtimeStatePath: filepath.Join(t.TempDir(), "runtime.yaml"),
		schedulerWake:    make(chan struct{}, 1),
	}
	manual := 18.5
	app.modeState = config.HeatingRuntimeState{
		Mode: config.HeatingModeBoost,
		Boost: &config.HeatingBoostState{
			TargetCelsius:             22.0,
			ExpiresAt:                 time.Now().UTC().Add(time.Hour),
			ResumeMode:                config.HeatingModeManual,
			ResumeManualTargetCelsius: &manual,
		},
	}

	state, err := app.CancelHeatingModeBoost(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != config.HeatingModeManual {
		t.Fatalf("got mode %q", state.Mode)
	}
	if state.Boost != nil {
		t.Fatalf("expected boost to be cleared, got %#v", state.Boost)
	}
	if state.ManualTargetCelsius == nil || *state.ManualTargetCelsius != manual {
		t.Fatalf("unexpected manual target %#v", state.ManualTargetCelsius)
	}
	if adapter.ensureOnCalls != 1 {
		t.Fatalf("got ensureOnCalls=%d", adapter.ensureOnCalls)
	}
	if len(adapter.setTargetCalls) != 1 || adapter.setTargetCalls[0] != manual {
		t.Fatalf("unexpected set target calls %#v", adapter.setTargetCalls)
	}
}
