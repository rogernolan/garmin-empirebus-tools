package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainlights "empirebus-tests/service/domains/lights"
)

var ErrLightsFlashInProgress = errors.New("lights flash already in progress")
var ErrInvalidFlashCount = errors.New("invalid flash count")

const exteriorFlashInterval = 500 * time.Millisecond

func recordExteriorSignal(state domainlights.State, on bool, at time.Time) domainlights.State {
	state.ExternalKnown = true
	state.ExternalOn = on
	state.LastUpdatedAt = &at
	return state
}

func (a *App) FlashExteriorLights(ctx context.Context, count int) (err error) {
	if count < 1 || count > 5 {
		a.setLightsCommandError(ErrInvalidFlashCount.Error())
		return ErrInvalidFlashCount
	}
	if a.lights == nil {
		err = fmt.Errorf("lights controller not configured")
		a.setLightsCommandError(err.Error())
		return err
	}
	sleep := a.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	a.mu.Lock()
	if a.lightsState.FlashInProgress {
		a.mu.Unlock()
		a.setLightsCommandError(ErrLightsFlashInProgress.Error())
		return ErrLightsFlashInProgress
	}
	a.lightsState.FlashInProgress = true
	a.lightsState.LastCommandError = ""
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.lightsState.FlashInProgress = false
		if err != nil {
			a.lightsState.LastCommandError = err.Error()
		} else {
			a.lightsState.LastCommandError = ""
		}
		a.mu.Unlock()
	}()

	snapshot := a.lights.LightsState()
	restore := a.lights.EnsureExteriorOff
	if snapshot.ExternalKnown && snapshot.ExternalOn {
		restore = a.lights.EnsureExteriorOn
	}

	defer func() {
		if restoreErr := restore(ctx); err == nil {
			err = restoreErr
		}
	}()

	for i := 0; i < count; i++ {
		if err = a.lights.EnsureExteriorOn(ctx); err != nil {
			return err
		}
		sleep(exteriorFlashInterval)
		if err = a.lights.EnsureExteriorOff(ctx); err != nil {
			return err
		}
		sleep(exteriorFlashInterval)
	}

	return nil
}

func (a *App) setLightsCommandError(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lightsState.LastCommandError = message
}
