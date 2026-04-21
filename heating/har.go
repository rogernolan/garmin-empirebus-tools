package heating

import (
	"encoding/json"
	"os"
	"time"
)

type harPayload struct {
	Log struct {
		Entries []struct {
			WebSocketMessages []struct {
				Type string  `json:"type"`
				Data string  `json:"data"`
				Time float64 `json:"time"`
			} `json:"_webSocketMessages"`
		} `json:"entries"`
	} `json:"log"`
}

func LoadHARFrames(path string) ([]Frame, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload harPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	best := payload.Log.Entries[0].WebSocketMessages
	for _, entry := range payload.Log.Entries[1:] {
		if len(entry.WebSocketMessages) > len(best) {
			best = entry.WebSocketMessages
		}
	}
	frames := make([]Frame, 0, len(best))
	base := time.Unix(0, 0)
	for idx, item := range best {
		wire, err := ParseWireFrame(item.Data)
		if err != nil {
			continue
		}
		dir := Direction(item.Type)
		frames = append(frames, Frame{
			At:        base.Add(time.Duration(idx) * time.Millisecond),
			Direction: dir,
			Wire:      wire,
		})
	}
	return frames, nil
}

func ReplayFrames(frames []Frame) HeaterState {
	var state HeaterState
	for _, frame := range frames {
		updateState(&state, frame)
	}
	return state
}
