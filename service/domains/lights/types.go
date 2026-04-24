package lights

import "time"

type State struct {
	ExternalKnown    bool       `json:"external_known"`
	ExternalOn       bool       `json:"external_on"`
	FlashInProgress  bool       `json:"flash_in_progress"`
	LastCommandError string     `json:"last_command_error,omitempty"`
	LastUpdatedAt    *time.Time `json:"last_updated_at,omitempty"`
}
