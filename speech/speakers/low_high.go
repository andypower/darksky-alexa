package speakers

import (
	"fmt"
	"sort"
	"time"

	"github.com/apex/log"
	"github.com/blockloop/darksky-alexa/alexa"
	"github.com/blockloop/darksky-alexa/darksky"
	"github.com/blockloop/darksky-alexa/pollen"
	"github.com/blockloop/darksky-alexa/tz"
	nowutil "github.com/jinzhu/now"
)

// LowHigh responds to the following queries
//
// the low|high [day] [time]
// the low|high
type LowHigh struct{}

var _ Speaker = LowHigh{}

func (LowHigh) Name() string {
	return "LowHigh"
}

func (lh LowHigh) CanSpeak(q *alexa.WeatherRequest) bool {
	return q.Condition == condHigh || q.Condition == condLow
}

func (lh LowHigh) Speak(loc *tz.Location, f *darksky.Forecast, _ *pollen.Forecast, q *alexa.WeatherRequest) string {
	if !lh.CanSpeak(q) {
		log.Error("tried to speak low/high without asking for low/high")
		return "a problem occurred"
	}

	start := nowutil.New(q.Start).BeginningOfDay().Add(-time.Nanosecond)
	end := nowutil.New(q.End).EndOfDay()
	dps := darksky.Where(f.Daily.Data, func(dp darksky.DataPoint) bool {
		t := dp.Time.Time()
		return t.Before(end) && t.After(start)
	})
	if len(dps) == 0 {
		return NoData
	}

	switch q.Condition {
	case condLow:
		dp := findLow(dps)
		return fmt.Sprintf("%.0f is the %s %s", dp.TemperatureLow, q.Condition, humanDay(dp.Time.Time()))
	case condHigh:
		dp := findHigh(dps)
		return fmt.Sprintf("%.0f is the %s %s", dp.TemperatureHigh, q.Condition, humanDay(dp.Time.Time()))
	default:
		log.Error("tried to speak low/high without asking for low/high")
		return "a problem occurred"
	}
}

func findLow(dps []darksky.DataPoint) *darksky.DataPoint {
	sort.SliceStable(dps, func(i, j int) bool {
		return dps[i].TemperatureLow < dps[j].TemperatureLow
	})
	return &dps[0]
}

func findHigh(dps []darksky.DataPoint) *darksky.DataPoint {
	sort.SliceStable(dps, func(i, j int) bool {
		return dps[i].TemperatureHigh > dps[j].TemperatureHigh
	})
	return &dps[0]
}
