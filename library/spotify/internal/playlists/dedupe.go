package playlists

import "sort"

type TrackInstance struct {
	PlaylistID string `json:"playlist_id"`
	TrackID    string `json:"track_id"`
	TrackURI   string `json:"track_uri"`
	Position   int    `json:"position"`
}

type DuplicateRemoval struct {
	TrackID  string `json:"track_id"`
	TrackURI string `json:"track_uri"`
	Position int    `json:"position"`
	Reason   string `json:"reason"`
}

func DedupePlan(items []TrackInstance, keep string) []DuplicateRemoval {
	if keep == "" {
		keep = "first"
	}
	byTrack := map[string][]TrackInstance{}
	for _, item := range items {
		if item.TrackID == "" {
			continue
		}
		byTrack[item.TrackID] = append(byTrack[item.TrackID], item)
	}
	var removals []DuplicateRemoval
	for trackID, instances := range byTrack {
		if len(instances) < 2 {
			continue
		}
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].Position < instances[j].Position
		})
		keepIndex := 0
		if keep == "last" {
			keepIndex = len(instances) - 1
		}
		for i, instance := range instances {
			if i == keepIndex {
				continue
			}
			removals = append(removals, DuplicateRemoval{
				TrackID:  trackID,
				TrackURI: instance.TrackURI,
				Position: instance.Position,
				Reason:   "same Spotify track ID appears more than once",
			})
		}
	}
	sort.Slice(removals, func(i, j int) bool {
		return removals[i].Position < removals[j].Position
	})
	return removals
}
