package water

import "time"

type ValveDirection string

const (
	ValveDirectionNone    ValveDirection = ""
	ValveDirectionOpening ValveDirection = "opening"
	ValveDirectionClosing ValveDirection = "closing"
)

type State struct {
	ValveKnown        bool           `json:"valve_known"`
	ValveMoving       bool           `json:"valve_moving"`
	ValveDirection    ValveDirection `json:"valve_direction,omitempty"`
	CommandInProgress bool           `json:"command_in_progress"`
	LastCommandError  string         `json:"last_command_error,omitempty"`
	LastUpdatedAt     *time.Time     `json:"last_updated_at,omitempty"`
}
