package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainwater "empirebus-tests/service/domains/water"
)

var ErrWaterCommandInProgress = errors.New("water command already in progress")

const greyWaterValveHoldDuration = 5 * time.Second

func (a *App) WaterState() domainwater.State {
	var state domainwater.State
	if a.water != nil {
		state = a.water.WaterState()
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	state.CommandInProgress = a.waterState.CommandInProgress
	state.LastCommandError = a.waterState.LastCommandError
	return state
}

func (a *App) OpenGreyWaterValve(ctx context.Context) error {
	return a.holdGreyWaterValve(ctx, domainwater.ValveDirectionOpening)
}

func (a *App) CloseGreyWaterValve(ctx context.Context) error {
	return a.holdGreyWaterValve(ctx, domainwater.ValveDirectionClosing)
}

func (a *App) holdGreyWaterValve(ctx context.Context, direction domainwater.ValveDirection) (err error) {
	if a.water == nil {
		err = fmt.Errorf("water controller not configured")
		a.setWaterCommandError(err.Error())
		return err
	}
	a.mu.Lock()
	if a.waterState.CommandInProgress {
		a.mu.Unlock()
		a.setWaterCommandError(ErrWaterCommandInProgress.Error())
		return ErrWaterCommandInProgress
	}
	a.waterState.CommandInProgress = true
	a.waterState.LastCommandError = ""
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.waterState.CommandInProgress = false
		if err != nil {
			a.waterState.LastCommandError = err.Error()
		} else {
			a.waterState.LastCommandError = ""
		}
		a.mu.Unlock()
	}()

	switch direction {
	case domainwater.ValveDirectionOpening:
		return a.water.OpenGreyWaterValve(ctx, greyWaterValveHoldDuration)
	case domainwater.ValveDirectionClosing:
		return a.water.CloseGreyWaterValve(ctx, greyWaterValveHoldDuration)
	default:
		err = fmt.Errorf("unsupported water valve direction %q", direction)
		return err
	}
}

func (a *App) setWaterCommandError(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waterState.LastCommandError = message
}
