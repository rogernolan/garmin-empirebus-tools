package heating

import (
	"fmt"
	"math"
	"time"
)

type PowerState string

const (
	PowerUnknown    PowerState = "unknown"
	PowerOff        PowerState = "off"
	PowerOn         PowerState = "on"
	PowerTransition PowerState = "transition"
)

type Evidence string

const (
	EvidenceUnknown       Evidence = "unknown"
	EvidenceSignal101     Evidence = "signal101"
	EvidenceSignal105     Evidence = "signal105"
	EvidenceSignal102     Evidence = "signal102"
	EvidenceSignal119     Evidence = "signal119"
	EvidenceCorrelatedAck Evidence = "correlated-ack"
)

type HeaterState struct {
	PowerState         PowerState
	PowerEvidence      Evidence
	BusyKnown          bool
	Busy               bool
	BusyEvidence       Evidence
	PumpKnown          bool
	PumpRunning        bool
	PumpEvidence       Evidence
	TargetTempKnown    bool
	TargetTempC        float64
	TargetRaw          int
	TargetEvidence     Evidence
	TargetPayload      []int
	LastUpdated        time.Time
	LastHeatingFrameAt time.Time
}

func (s HeaterState) Ready() bool {
	if s.PowerState != PowerOn {
		return false
	}
	return s.BusyKnown && !s.Busy
}

func (s HeaterState) Clone() HeaterState {
	dup := s
	if s.TargetPayload != nil {
		dup.TargetPayload = append([]int(nil), s.TargetPayload...)
	}
	return dup
}

func (s HeaterState) String() string {
	target := "unknown"
	if s.TargetTempKnown {
		target = fmt.Sprintf("%.1fC", s.TargetTempC)
	}
	return fmt.Sprintf(
		"power=%s busy=%t pump=%t target=%s raw=%d",
		s.PowerState,
		s.Busy,
		s.PumpRunning,
		target,
		s.TargetRaw,
	)
}

func updateState(state *HeaterState, frame Frame) bool {
	changed := false
	state.LastUpdated = frame.At
	if frame.RelevantToHeating() {
		state.LastHeatingFrameAt = frame.At
	}
	data := frame.Wire.Data
	if len(data) < 3 {
		return false
	}
	switch frame.SignalID() {
	case SignalHeatingPower:
		next := PowerUnknown
		switch data[2] {
		case 0:
			next = PowerOff
		case 1:
			next = PowerOn
		case 129:
			next = PowerTransition
		}
		if next != PowerUnknown && state.PowerState != next {
			state.PowerState = next
			state.PowerEvidence = EvidenceSignal101
			changed = true
		}
	case SignalHeatingBusy:
		value := data[2] == 1
		if !state.BusyKnown || state.Busy != value {
			state.BusyKnown = true
			state.Busy = value
			state.BusyEvidence = EvidenceSignal102
			changed = true
		}
	case SignalHeatingPump:
		value := data[2] == 1
		if !state.PumpKnown || state.PumpRunning != value {
			state.PumpKnown = true
			state.PumpRunning = value
			state.PumpEvidence = EvidenceSignal119
			changed = true
		}
	case SignalHeatingTargetTemp:
		raw, tempC, ok := decodeTargetTemperature(data)
		if ok && (!state.TargetTempKnown || state.TargetRaw != raw || math.Abs(state.TargetTempC-tempC) > 0.001) {
			state.TargetTempKnown = true
			state.TargetTempC = tempC
			state.TargetRaw = raw
			state.TargetEvidence = EvidenceSignal105
			state.TargetPayload = append([]int(nil), data...)
			changed = true
		}
	}
	return changed
}

func decodeTargetTemperature(data []int) (int, float64, bool) {
	if len(data) < 6 || data[0] != SignalHeatingTargetTemp {
		return 0, 0, false
	}
	raw := data[4] + (data[5] << 8)
	bestTemp := 0.0
	bestDelta := math.MaxInt
	for step := 0; step <= 80; step++ {
		temp := float64(step) / 2
		expected := encodeTargetTemperature(temp)
		delta := expected - raw
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			bestDelta = delta
			bestTemp = temp
		}
	}
	if bestDelta > 20 {
		return raw, 0, false
	}
	return raw, bestTemp, true
}

func encodeTargetTemperature(tempC float64) int {
	wholeTens := int(math.Floor(tempC / 10.0))
	return 10956 + int(math.Round(tempC*1000)) + (wholeTens * 10)
}
