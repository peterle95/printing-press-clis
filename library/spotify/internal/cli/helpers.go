package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/config"
	"spotify-pp-cli/internal/library"
	"spotify-pp-cli/internal/rules"
	"spotify-pp-cli/internal/store"
)

type trackRef struct {
	ID    string
	URI   string
	Query string
}

func parseTrackRef(input string) trackRef {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "spotify:track:") {
		id := strings.TrimPrefix(input, "spotify:track:")
		return trackRef{ID: id, URI: input}
	}
	if strings.Contains(input, "open.spotify.com/track/") {
		u, err := url.Parse(input)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			for i, part := range parts {
				if part == "track" && i+1 < len(parts) {
					id := parts[i+1]
					return trackRef{ID: id, URI: "spotify:track:" + id}
				}
			}
		}
	}
	if spotifyID.MatchString(input) {
		return trackRef{ID: input, URI: "spotify:track:" + input}
	}
	return trackRef{Query: input}
}

func loadRulesFile(path string) (*rules.File, string, error) {
	if path == "" {
		var err error
		path, err = configRulesPath()
		if err != nil {
			return nil, "", err
		}
	}
	rf, err := rules.Load(path)
	if err != nil {
		return nil, path, fmt.Errorf("load rules from %s: %w (run 'spotify-pp-cli rules init' first)", path, err)
	}
	if errs := rules.Validate(rf); len(errs) > 0 {
		return nil, path, errs[0]
	}
	return rf, path, nil
}

func classifyTrack(ctx context.Context, st *store.Store, ruleFile *rules.File, track store.TrackRecord) (classifier.Result, error) {
	artists, err := st.ArtistsForTrack(ctx, track.ID)
	if err != nil {
		return classifier.Result{}, err
	}
	result := classifier.New(ruleFile).Classify(track, artists)
	return result, st.UpsertClassification(ctx, store.ClassificationRecord{
		TrackID:            result.TrackID,
		PrimaryGenre:       result.PrimaryGenre,
		MatchedRule:        result.MatchedRule,
		Confidence:         result.Confidence,
		TargetPlaylistID:   result.TargetPlaylistID,
		TargetPlaylistName: result.TargetPlaylistName,
		Explanation:        result.Explanation,
		UpdatedAt:          result.UpdatedAt.Format(time.RFC3339),
	})
}

func classifyAll(ctx context.Context, st *store.Store, ruleFile *rules.File, onlyUnclassified bool) ([]store.TrackRecord, map[string]classifier.Result, error) {
	tracks, err := st.LikedTracks(ctx)
	if err != nil {
		return nil, nil, err
	}
	existing, err := st.Classifications(ctx)
	if err != nil {
		return nil, nil, err
	}
	results := map[string]classifier.Result{}
	for _, track := range tracks {
		if onlyUnclassified {
			if _, ok := existing[track.ID]; ok {
				continue
			}
		}
		result, err := classifyTrack(ctx, st, ruleFile, track)
		if err != nil {
			return nil, nil, err
		}
		results[track.ID] = result
	}
	if onlyUnclassified {
		filtered := tracks[:0]
		for _, track := range tracks {
			if _, ok := results[track.ID]; ok {
				filtered = append(filtered, track)
			}
		}
		tracks = filtered
	} else {
		for _, rec := range existing {
			if _, ok := results[rec.TrackID]; !ok {
				results[rec.TrackID] = classifier.Result{
					TrackID:            rec.TrackID,
					PrimaryGenre:       rec.PrimaryGenre,
					MatchedRule:        rec.MatchedRule,
					Confidence:         rec.Confidence,
					TargetPlaylistID:   rec.TargetPlaylistID,
					TargetPlaylistName: rec.TargetPlaylistName,
					Explanation:        rec.Explanation,
				}
			}
		}
	}
	return tracks, results, nil
}

func resolveTrack(ctx context.Context, app *app, input string) (store.TrackRecord, error) {
	ref := parseTrackRef(input)
	if ref.ID != "" || ref.URI != "" {
		if track, ok, err := app.Store.TrackByIDOrURI(ctx, firstNonEmpty(ref.ID, ref.URI)); err != nil {
			return store.TrackRecord{}, err
		} else if ok {
			return track, nil
		}
		if ref.ID != "" {
			apiTrack, err := app.Client.GetTrack(ctx, ref.ID, app.Config.Market)
			if err != nil {
				return store.TrackRecord{}, err
			}
			track := library.TrackRecord(apiTrack, "")
			if err := app.Store.UpsertTrack(ctx, track); err != nil {
				return store.TrackRecord{}, err
			}
			return track, nil
		}
	}
	local, err := app.Store.SearchTracksLocal(ctx, ref.Query, 2)
	if err != nil {
		return store.TrackRecord{}, err
	}
	if len(local) == 1 {
		return local[0], nil
	}
	apiTracks, err := app.Client.SearchTracks(ctx, ref.Query, 5, app.Config.Market)
	if err != nil {
		return store.TrackRecord{}, err
	}
	if len(apiTracks) == 0 {
		return store.TrackRecord{}, fmt.Errorf("no track found for %q", input)
	}
	track := library.TrackRecord(apiTracks[0], "")
	if err := app.Store.UpsertTrack(ctx, track); err != nil {
		return store.TrackRecord{}, err
	}
	return track, nil
}

func confirm(flags *rootFlags, required bool, phrase string) error {
	if !required || flags.yes {
		return nil
	}
	fmt.Printf("Type the exact phrase to continue: %s\n> ", phrase)
	reader := bufio.NewReader(os.Stdin)
	got, _ := reader.ReadString('\n')
	if strings.TrimSpace(got) != phrase {
		return fmt.Errorf("confirmation phrase did not match; no changes made")
	}
	return nil
}

func printCSV(rows [][]string) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("operation id must be a positive integer")
	}
	return id, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func configRulesPath() (string, error) {
	return config.RulesPath()
}

var spotifyID = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
