package organizer

import (
	"encoding/json"
	"sort"
	"time"

	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/store"
)

type PlanItem struct {
	Track          store.TrackRecord `json:"track"`
	Classification classifier.Result `json:"classification"`
	Action         string            `json:"action"`
	TargetPlaylist string            `json:"target_playlist"`
	Reason         string            `json:"reason"`
}

type Plan struct {
	OperationType      string            `json:"operation_type"`
	DryRun             bool              `json:"dry_run"`
	TracksScanned      int               `json:"tracks_scanned"`
	Classified         int               `json:"classified"`
	NeedsReview        int               `json:"needs_review"`
	RemoveAfterAdd     bool              `json:"remove_after_add"`
	ByPlaylist         map[string]int    `json:"by_playlist"`
	ByConfidence       map[string]int    `json:"by_confidence"`
	Items              []PlanItem        `json:"items"`
	GeneratedAt        time.Time         `json:"generated_at"`
	ExistingPlaylistID map[string]string `json:"existing_playlist_id,omitempty"`
}

type UndoPlan struct {
	OperationID    int64               `json:"operation_id"`
	AddToLibrary   []string            `json:"add_to_library,omitempty"`
	RemovePlaylist map[string][]string `json:"remove_playlist,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

func BuildPlan(operationType string, dryRun bool, tracks []store.TrackRecord, results map[string]classifier.Result) Plan {
	plan := Plan{
		OperationType: operationType,
		DryRun:        dryRun,
		TracksScanned: len(tracks),
		ByPlaylist:    map[string]int{},
		ByConfidence:  map[string]int{},
		GeneratedAt:   time.Now().UTC(),
	}
	for _, track := range tracks {
		result, ok := results[track.ID]
		if !ok {
			result = classifier.Result{
				TrackID:            track.ID,
				TrackURI:           track.URI,
				TrackName:          track.Name,
				TargetPlaylistName: "Needs Review",
				MatchedRule:        "fallback",
				Confidence:         classifier.ConfidenceNone,
				Explanation:        "not classified",
			}
		}
		if result.Confidence == classifier.ConfidenceNone || result.Confidence == classifier.ConfidenceLow {
			plan.NeedsReview++
		} else {
			plan.Classified++
		}
		plan.ByPlaylist[result.TargetPlaylistName]++
		plan.ByConfidence[result.Confidence]++
		plan.Items = append(plan.Items, PlanItem{
			Track:          track,
			Classification: result,
			Action:         operationType,
			TargetPlaylist: result.TargetPlaylistName,
			Reason:         result.Explanation,
		})
	}
	sort.SliceStable(plan.Items, func(i, j int) bool {
		if plan.Items[i].TargetPlaylist == plan.Items[j].TargetPlaylist {
			return plan.Items[i].Track.Name < plan.Items[j].Track.Name
		}
		return plan.Items[i].TargetPlaylist < plan.Items[j].TargetPlaylist
	})
	return plan
}

func BatchStrings(values []string, size int) [][]string {
	if size <= 0 {
		return nil
	}
	var out [][]string
	for len(values) > 0 {
		n := size
		if len(values) < n {
			n = len(values)
		}
		chunk := append([]string(nil), values[:n]...)
		out = append(out, chunk)
		values = values[n:]
	}
	return out
}

func BuildUndoPlan(op store.OperationRecord, items []store.OperationItemRecord) UndoPlan {
	plan := UndoPlan{
		OperationID:    op.ID,
		RemovePlaylist: map[string][]string{},
	}
	for _, item := range items {
		if item.Status != "success" || item.TrackURI == "" {
			continue
		}
		switch item.Action {
		case "playlist_add":
			plan.RemovePlaylist[item.TargetPlaylistID] = append(plan.RemovePlaylist[item.TargetPlaylistID], item.TrackURI)
		case "library_remove":
			plan.AddToLibrary = append(plan.AddToLibrary, item.TrackURI)
		}
	}
	if len(plan.RemovePlaylist) > 0 {
		plan.Warnings = append(plan.Warnings, "playlist removals use current playlist state and may not preserve original ordering")
	}
	return plan
}

func SummaryJSON(plan Plan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return "{}"
	}
	return string(b)
}
