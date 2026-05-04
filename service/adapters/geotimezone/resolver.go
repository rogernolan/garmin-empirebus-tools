package geotimezone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Resolver struct {
	endpoint string
	client   *http.Client
}

func New(endpoint string, timeout time.Duration) *Resolver {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "https://api.geotimezone.com/public/timezone"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Resolver{
		endpoint: endpoint,
		client:   &http.Client{Timeout: timeout},
	}
}

func (r *Resolver) Timezone(ctx context.Context, latitude, longitude float64) (string, error) {
	endpoint, err := url.Parse(r.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse timezone endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("latitude", fmt.Sprintf("%.6f", latitude))
	query.Set("longitude", fmt.Sprintf("%.6f", longitude))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload struct {
		IANA string `json:"iana_timezone"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode timezone response: %w", err)
	}
	if strings.TrimSpace(payload.IANA) == "" {
		return "", fmt.Errorf("timezone response missing iana_timezone")
	}
	if _, err := time.LoadLocation(payload.IANA); err != nil {
		return "", fmt.Errorf("timezone response invalid iana_timezone %q: %w", payload.IANA, err)
	}
	return payload.IANA, nil
}
