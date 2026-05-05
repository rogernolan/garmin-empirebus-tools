package water

import "time"

type ValveDirection string

const (
	ValveDirectionNone    ValveDirection = ""
	ValveDirectionOpening ValveDirection = "opening"
	ValveDirectionClosing ValveDirection = "closing"
)

type State struct {
	ValveKnown              bool              `json:"valve_known"`
	ValveMoving             bool              `json:"valve_moving"`
	ValveDirection          ValveDirection    `json:"valve_direction,omitempty"`
	CommandInProgress       bool              `json:"command_in_progress"`
	LastCommandError        string            `json:"last_command_error,omitempty"`
	ScheduledOpening        *ScheduledOpening `json:"scheduled_opening,omitempty"`
	LastScheduleMessage     string            `json:"last_schedule_message,omitempty"`
	LastScheduleCompletedAt *time.Time        `json:"last_schedule_completed_at,omitempty"`
	LastUpdatedAt           *time.Time        `json:"last_updated_at,omitempty"`
}

type ScheduledOpening struct {
	OpenAt          time.Time  `json:"open_at"`
	LocalTime       string     `json:"local_time"`
	Timezone        string     `json:"timezone"`
	DurationMinutes int        `json:"duration_minutes"`
	Status          string     `json:"status"`
	OpenedAt        *time.Time `json:"opened_at,omitempty"`
}
