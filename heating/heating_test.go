package heating

import (
	"path/filepath"
	"testing"
)

func TestDecodeTargetTemperatureSamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []int
		want float64
	}{
		{"8.0", []int{105, 0, 0, 22, 12, 74, 4, 0}, 8.0},
		{"10.0", []int{105, 0, 0, 22, 230, 81, 4, 0}, 10.0},
		{"13.0", []int{105, 0, 0, 22, 158, 93, 4, 0}, 13.0},
		{"20.0", []int{105, 0, 0, 22, 0, 121, 4, 0}, 20.0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, got, ok := decodeTargetTemperature(tc.data)
			if !ok {
				t.Fatalf("decode failed for %v", tc.data)
			}
			if got != tc.want {
				t.Fatalf("got %.1f want %.1f", got, tc.want)
			}
		})
	}
}

func TestReplayHeatingHAR(t *testing.T) {
	t.Parallel()
	frames, err := LoadHARFrames(filepath.Join("..", "Heating.har"))
	if err != nil {
		t.Fatal(err)
	}
	state := ReplayFrames(frames)
	if !state.TargetTempKnown {
		t.Fatal("expected target temperature to be known")
	}
	if state.TargetTempC != 8.0 {
		t.Fatalf("got %.1f want 8.0", state.TargetTempC)
	}
	if state.PowerState != PowerOff {
		t.Fatalf("got power %s want off", state.PowerState)
	}
}

func TestReplayHeating20CHAR(t *testing.T) {
	t.Parallel()
	frames, err := LoadHARFrames(filepath.Join("..", "Load with Heating on at 20C.har"))
	if err != nil {
		t.Fatal(err)
	}
	state := ReplayFrames(frames)
	if !state.TargetTempKnown || state.TargetTempC != 20.0 {
		t.Fatalf("got target %.1f known=%t want 20.0", state.TargetTempC, state.TargetTempKnown)
	}
	if state.PowerState != PowerOn {
		t.Fatalf("got power %s want on", state.PowerState)
	}
	if !state.Ready() {
		t.Fatalf("expected ready state, got %s", state.String())
	}
}

func TestReplayHeatingSweepHAR(t *testing.T) {
	t.Parallel()
	frames, err := LoadHARFrames(filepath.Join("..", "Heating 13C-20C.har"))
	if err != nil {
		t.Fatal(err)
	}
	state := ReplayFrames(frames)
	if !state.TargetTempKnown || state.TargetTempC != 20.0 {
		t.Fatalf("got target %.1f known=%t want 20.0", state.TargetTempC, state.TargetTempKnown)
	}
	if state.PowerState != PowerOff {
		t.Fatalf("got power %s want off", state.PowerState)
	}
}
