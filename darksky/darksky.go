// Package darksky provides functionality for interacting with the darksky
// API. For more information see https://darksky.net/dev/docs
package darksky

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

var defaultHTTPClient = &http.Client{
	Timeout: time.Second * 5,
}

func New(token string) *API {
	return NewWithClient(token, defaultHTTPClient)
}

func NewWithClient(token string, client *http.Client) *API {
	if token == "" {
		panic("token cannot be empty")
	}
	return &API{
		token:  token,
		client: client,
	}
}

type API struct {
	token  string
	client *http.Client
}

// GetForecast retrieves the forecast for the given latitude and longitude
func (api *API) GetForecast(ctx context.Context, lat, lon string) (*Forecast, error) {
	url := fmt.Sprintf("https://api.darksky.net/forecast/%s/%s,%s", api.token, lat, lon)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate new http request")
	}

	resp, err := api.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get darksky forecast")
	}
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.Errorf("bad status code from darksky (%d). body: %s", resp.StatusCode, body)
	}

	var forecast Forecast
	err = json.NewDecoder(resp.Body).Decode(&forecast)
	return &forecast, errors.Wrap(err, "failed to parse response from darksky")
}
