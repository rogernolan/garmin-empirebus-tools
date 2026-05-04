package tzfresolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ringsaturn/tzf"
)

type finder interface {
	GetTimezoneName(lng float64, lat float64) string
}

type Resolver struct {
	finder finder
}

func New() (*Resolver, error) {
	finder, err := tzf.NewDefaultFinder()
	if err != nil {
		return nil, err
	}
	return &Resolver{finder: finder}, nil
}

func NewWithFinder(f finder) *Resolver {
	return &Resolver{finder: f}
}

func (r *Resolver) Timezone(ctx context.Context, latitude, longitude float64) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	timezoneName := strings.TrimSpace(r.finder.GetTimezoneName(longitude, latitude))
	if timezoneName == "" {
		return "", fmt.Errorf("tzf timezone lookup returned no timezone")
	}
	if _, err := time.LoadLocation(timezoneName); err != nil {
		return "", fmt.Errorf("tzf timezone lookup returned invalid timezone %q: %w", timezoneName, err)
	}
	return timezoneName, nil
}
