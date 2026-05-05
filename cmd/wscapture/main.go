package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"empirebus-tests/heating"

	"github.com/gorilla/websocket"
)

type captureRecord struct {
	At         time.Time          `json:"at"`
	Direction  string             `json:"direction"`
	Message    string             `json:"message,omitempty"`
	MessageLen int                `json:"message_len,omitempty"`
	Frame      *heating.WireFrame `json:"frame,omitempty"`
	Signal     *int               `json:"signal,omitempty"`
	Value      *int               `json:"value,omitempty"`
	Error      string             `json:"error,omitempty"`
}

type recorder struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func main() {
	var (
		wsURL             = flag.String("ws-url", heating.DefaultWSURL, "websocket URL")
		origin            = flag.String("origin", "", "origin header")
		outPath           = flag.String("out", "", "output NDJSON file")
		duration          = flag.Duration("duration", 0, "capture duration; 0 runs until interrupted")
		heartbeatInterval = flag.Duration("heartbeat-interval", 4*time.Second, "heartbeat interval")
		noBootstrap       = flag.Bool("no-bootstrap", false, "do not send Garmin subscription bootstrap messages")
		noHeartbeat       = flag.Bool("no-heartbeat", false, "do not send heartbeat messages")
		includeRaw        = flag.Bool("raw", true, "include raw websocket messages in output")
	)
	flag.Parse()

	if strings.TrimSpace(*outPath) == "" {
		*outPath = fmt.Sprintf("captures/garmin-ws-%s.ndjson", time.Now().Format("20060102-150405"))
	}
	if err := run(*wsURL, *origin, *outPath, *duration, *heartbeatInterval, !*noBootstrap, !*noHeartbeat, *includeRaw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(wsURL, origin, outPath string, duration, heartbeatInterval time.Duration, bootstrap, heartbeat, includeRaw bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	if err := os.MkdirAll(parentDir(outPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()
	rec := &recorder{encoder: json.NewEncoder(out)}

	headers := http.Header{}
	if origin != "" {
		headers.Set("Origin", origin)
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return fmt.Errorf("connect websocket: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(os.Stderr, "capturing %s to %s\n", wsURL, outPath)
	if bootstrap {
		for _, message := range heating.DefaultBootstrapMessages {
			if err := writeMessage(conn, rec, "send", message, includeRaw); err != nil {
				return err
			}
		}
	}

	errCh := make(chan error, 2)
	if heartbeat {
		go heartbeatLoop(ctx, conn, rec, heartbeatInterval, includeRaw, errCh)
	}
	go readLoop(conn, rec, includeRaw, errCh)

	select {
	case <-ctx.Done():
		_ = rec.write(captureRecord{At: time.Now(), Direction: "event", Message: ctx.Err().Error()})
		return nil
	case err := <-errCh:
		_ = rec.write(captureRecord{At: time.Now(), Direction: "event", Error: err.Error()})
		return err
	}
}

func heartbeatLoop(ctx context.Context, conn *websocket.Conn, rec *recorder, interval time.Duration, includeRaw bool, errCh chan<- error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := writeMessage(conn, rec, "send", heating.DefaultHeartbeatMessage, includeRaw); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func readLoop(conn *websocket.Conn, rec *recorder, includeRaw bool, errCh chan<- error) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			errCh <- fmt.Errorf("read websocket: %w", err)
			return
		}
		direction := "receive"
		if messageType != websocket.TextMessage {
			direction = fmt.Sprintf("receive:%d", messageType)
		}
		if err := writeRecordForMessage(rec, direction, string(payload), includeRaw); err != nil {
			errCh <- err
			return
		}
	}
}

func writeMessage(conn *websocket.Conn, rec *recorder, direction, message string, includeRaw bool) error {
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		return fmt.Errorf("write websocket: %w", err)
	}
	return writeRecordForMessage(rec, direction, message, includeRaw)
}

func writeRecordForMessage(rec *recorder, direction, message string, includeRaw bool) error {
	record := captureRecord{
		At:         time.Now(),
		Direction:  direction,
		MessageLen: len(message),
	}
	if includeRaw {
		record.Message = message
	}
	if frame, err := heating.ParseWireFrame(message); err == nil {
		record.Frame = &frame
		if len(frame.Data) > 0 {
			signal := frame.Data[0]
			record.Signal = &signal
		}
		if len(frame.Data) > 2 {
			value := frame.Data[2]
			record.Value = &value
		}
	} else {
		record.Error = fmt.Sprintf("parse frame: %v", err)
	}
	return rec.write(record)
}

func (r *recorder) write(record captureRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.encoder.Encode(record); err != nil {
		return fmt.Errorf("write capture record: %w", err)
	}
	return nil
}

func parentDir(path string) string {
	dir := "."
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		dir = path[:slash]
	}
	if dir == "" {
		return "."
	}
	return dir
}
