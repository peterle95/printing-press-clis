package workouts

import (
	"fmt"
	"sort"
	"time"

	"hevy-pp-cli/internal/database"
)

type PR struct {
	Exercise, Metric, WorkoutID, Date string
	Value                             float64
}

func PRs(rows []database.SetRecord) []PR {
	var bestW, bestR, bestE, bestV *PR
	for _, s := range rows {
		if !s.HasWeight || !s.HasReps {
			continue
		}
		d := s.StartedAt
		candidate := func(metric string, v float64) *PR {
			return &PR{Exercise: s.Exercise, Metric: metric, WorkoutID: s.WorkoutTitle, Date: d, Value: v}
		}
		if bestW == nil || s.Weight > bestW.Value {
			bestW = candidate("highest_weight", s.Weight)
		}
		if bestR == nil || s.Reps > bestR.Value {
			bestR = candidate("highest_repetitions", s.Reps)
		}
		e := s.Weight * (1 + s.Reps/30)
		if bestE == nil || e > bestE.Value {
			bestE = candidate("estimated_1rm_epley", e)
		}
		v := s.Weight * s.Reps
		if bestV == nil || v > bestV.Value {
			bestV = candidate("highest_set_volume", v)
		}
	}
	out := []PR{}
	for _, x := range []*PR{bestW, bestR, bestE, bestV} {
		if x != nil {
			out = append(out, *x)
		}
	}
	return out
}

type Volume struct {
	Group  string  `json:"group"`
	Volume float64 `json:"volume"`
}

func Volumes(rows []database.SetRecord, group string) []Volume {
	m := map[string]float64{}
	for _, s := range rows {
		if !s.HasWeight || !s.HasReps {
			continue
		}
		t, _ := time.Parse(time.RFC3339, s.StartedAt)
		key := t.Format("2006-01-02")
		if group == "week" {
			y, w := t.ISOWeek()
			key = fmt.Sprintf("%04d-W%02d", y, w)
		}
		m[key] += s.Weight * s.Reps
	}
	o := make([]Volume, 0, len(m))
	for k, v := range m {
		o = append(o, Volume{k, v})
	}
	sort.Slice(o, func(i, j int) bool { return o[i].Group < o[j].Group })
	return o
}
