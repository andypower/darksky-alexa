package cache

import (
	"context"

	"github.com/apex/log"
	"github.com/blockloop/darksky-alexa/darksky"
	"github.com/pkg/errors"
)

// WriteThrough is a cache layer that has a fallback layer
type WriteThrough struct {
	cache Cache
	api   *darksky.API
}

// NewWriteThrough creates a new WriteThrough cache
func NewWriteThrough(cache Cache, api *darksky.API) *WriteThrough {
	return &WriteThrough{
		cache: cache,
		api:   api,
	}
}

// GetForecast first tries to retrieve a cached result and falls back to
// directly fetching to the API. If the API is used then results are
// stored in the cache store
func (w *WriteThrough) GetForecast(ctx context.Context, lat, lon string) (*darksky.Forecast, error) {
	ll := log.WithFields(log.Fields{
		"latitude":  lat,
		"longitude": lon,
	})

	cached, err := w.cache.GetForecast(ctx, lat, lon)
	if err != nil {
		ll.WithError(err).Error("failed to get cached forecast")
	}
	if cached != nil {
		ll.Info("cache hit")
		return cached, nil
	}
	ll.Info("cache miss")

	result, err := w.api.GetForecast(ctx, lat, lon)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch forecast from API")
	}
	if result != nil {
		go func(ll log.Interface) {
			err := w.cache.PutForecast(context.Background(), lat, lon, result)
			if err != nil {
				ll.WithError(err).Error("failed to put cache")
			}
		}(ll)
	}

	return result, nil
}
