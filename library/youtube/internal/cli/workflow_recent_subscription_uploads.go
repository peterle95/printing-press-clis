package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"youtube-pp-cli/internal/client"
	"youtube-pp-cli/internal/cliutil"
)

type subscribedChannel struct {
	ChannelID    string `json:"channel_id"`
	ChannelTitle string `json:"channel_title"`
	NewItemCount int    `json:"new_item_count,omitempty"`
}

type channelUploadsSource struct {
	ChannelID         string `json:"channel_id"`
	ChannelTitle      string `json:"channel_title"`
	UploadsPlaylistID string `json:"uploads_playlist_id"`
}

type recentSubscriptionUpload struct {
	VideoID           string `json:"video_id"`
	Title             string `json:"title"`
	ChannelID         string `json:"channel_id"`
	ChannelTitle      string `json:"channel_title"`
	PublishedAt       string `json:"published_at"`
	URL               string `json:"url"`
	ThumbnailURL      string `json:"thumbnail_url,omitempty"`
	UploadsPlaylistID string `json:"uploads_playlist_id,omitempty"`
	PlaylistStatus    string `json:"playlist_status,omitempty"`
	PlaylistItemID    string `json:"playlist_item_id,omitempty"`
}

type recentSubscriptionUploadsMeta struct {
	Source               string `json:"source"`
	Since                string `json:"since"`
	Cutoff               string `json:"cutoff"`
	SubscriptionsScanned int    `json:"subscriptions_scanned"`
	SubscriptionPages    int    `json:"subscription_pages"`
	ChannelsChecked      int    `json:"channels_checked"`
	ChannelBatches       int    `json:"channel_batches"`
	PerChannel           int    `json:"per_channel"`
	Concurrency          int    `json:"concurrency"`
	VideosFound          int    `json:"videos_found"`
	PlaylistID           string `json:"playlist_id,omitempty"`
	PlaylistTitle        string `json:"playlist_title,omitempty"`
	PlaylistCreated      bool   `json:"playlist_created,omitempty"`
	AddedToPlaylist      int    `json:"added_to_playlist,omitempty"`
	SkippedExisting      int    `json:"skipped_existing,omitempty"`
	DryRun               bool   `json:"dry_run,omitempty"`
	Note                 string `json:"note,omitempty"`
}

type recentSubscriptionUploadsOutput struct {
	Meta     recentSubscriptionUploadsMeta `json:"meta"`
	Results  []recentSubscriptionUpload    `json:"results"`
	Warnings []string                      `json:"warnings,omitempty"`
}

type youtubeListEnvelope struct {
	Items         []json.RawMessage `json:"items"`
	NextPageToken string            `json:"nextPageToken"`
}

type youtubeSubscriptionItem struct {
	Snippet struct {
		Title      string `json:"title"`
		ResourceID struct {
			ChannelID string `json:"channelId"`
		} `json:"resourceId"`
	} `json:"snippet"`
	ContentDetails struct {
		NewItemCount int `json:"newItemCount"`
	} `json:"contentDetails"`
}

type youtubeChannelItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title string `json:"title"`
	} `json:"snippet"`
	ContentDetails struct {
		RelatedPlaylists struct {
			Uploads string `json:"uploads"`
		} `json:"relatedPlaylists"`
	} `json:"contentDetails"`
}

type youtubePlaylistItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title                  string `json:"title"`
		PublishedAt            string `json:"publishedAt"`
		VideoOwnerChannelID    string `json:"videoOwnerChannelId"`
		VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
		ResourceID             struct {
			VideoID string `json:"videoId"`
		} `json:"resourceId"`
		Thumbnails map[string]youtubeThumbnail `json:"thumbnails"`
	} `json:"snippet"`
	ContentDetails struct {
		VideoID          string `json:"videoId"`
		VideoPublishedAt string `json:"videoPublishedAt"`
	} `json:"contentDetails"`
}

type youtubePlaylist struct {
	ID      string `json:"id"`
	Snippet struct {
		Title string `json:"title"`
	} `json:"snippet"`
}

type youtubeThumbnail struct {
	URL string `json:"url"`
}

type destinationPlaylist struct {
	Enabled bool
	ID      string
	Title   string
	Created bool
}

// PATCH: Add a YouTube-specific workflow for the notification-adjacent use
// case the Data API can support: recent uploads from subscribed channels.
func newWorkflowRecentSubscriptionUploadsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var subscriptionOrder string
	var perChannel int
	var concurrency int
	var maxSubscriptions int
	var limit int
	var playlistID string
	var playlistTitle string
	var createPlaylist bool
	var privacyStatus string
	var notificationScope string
	var channelIDFile string

	cmd := &cobra.Command{
		Use:     "recent-subscription-uploads",
		Aliases: []string{"notifications"},
		Short:   "Find recent uploads from subscribed channels and optionally add them to a playlist",
		Long: `Find recent uploads from the authenticated user's subscribed channels.

YouTube Data API v3 does not expose the web notification bell state or the
notifications tray. This workflow efficiently approximates that feed by listing
subscriptions, batching channel lookups to get upload playlists, then checking
the head of each uploads playlist in parallel.

When invoked as "workflow notifications" or with "--notification-scope all",
the command means channels whose YouTube web bell menu is set to All. The
Data API cannot discover that bell state, so that mode requires
--channel-id-file with channel IDs collected from the web UI.`,
		Example: `  # List uploads from subscribed channels in the last day
  youtube-pp-cli workflow recent-subscription-uploads --agent --since 24h

  # List uploads from channels whose YouTube web bell menu is set to All
  youtube-pp-cli workflow notifications --agent --since 24h --channel-id-file all-bell-channel-ids.txt

  # Add discovered videos to an existing playlist
  youtube-pp-cli workflow recent-subscription-uploads --since 24h --playlist-id PL... --agent

  # Preview playlist additions without mutating YouTube
  youtube-pp-cli workflow recent-subscription-uploads --since 24h --playlist-title "Daily subscriptions" --create-playlist --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if perChannel < 1 || perChannel > 50 {
				return usageErr(fmt.Errorf("--per-channel must be between 1 and 50"))
			}
			if concurrency < 1 {
				concurrency = 1
			}
			if playlistID != "" && playlistTitle != "" {
				return usageErr(fmt.Errorf("use either --playlist-id or --playlist-title, not both"))
			}
			if privacyStatus == "" {
				privacyStatus = "private"
			}
			// PATCH: Treat the web bell "All" state as an explicit external
			// filter. YouTube Data API subscriptions do not expose that field,
			// so silently using all subscriptions would be misleading.
			if notificationScope == "" {
				if cmd.CalledAs() == "notifications" {
					notificationScope = "all"
				} else {
					notificationScope = "subscriptions"
				}
			}
			switch notificationScope {
			case "subscriptions":
			case "all":
				resolved, err := resolveAllBellChannelIDFile(channelIDFile)
				if err != nil {
					return usageErr(err)
				}
				channelIDFile = resolved
			default:
				return usageErr(fmt.Errorf("--notification-scope must be subscriptions or all"))
			}

			cutoff, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			// PATCH: In this workflow, global --dry-run means "discover live
			// videos, then preview playlist mutations"; GET discovery must
			// still run so the preview is meaningful.
			if flags.dryRun {
				c.DryRun = false
			}

			subscriptions, subscriptionPages, err := fetchSubscribedChannels(c, subscriptionOrder, maxSubscriptions)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			channelFilter, err := loadChannelIDFilter(channelIDFile)
			if err != nil {
				return usageErr(err)
			}
			if channelFilter != nil {
				subscriptions = filterSubscribedChannels(subscriptions, channelFilter)
			}
			sources, channelBatches, err := fetchChannelUploadsSources(c, subscriptions)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			uploads, warnings := fetchRecentUploadsFromSources(cmd.Context(), c, sources, cutoff, perChannel, concurrency)
			sort.SliceStable(uploads, func(i, j int) bool {
				return uploads[i].PublishedAt > uploads[j].PublishedAt
			})
			if limit > 0 && len(uploads) > limit {
				uploads = uploads[:limit]
			}

			dest, err := resolveDestinationPlaylist(c, playlistID, playlistTitle, createPlaylist, privacyStatus, flags.dryRun)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			added, skipped := 0, 0
			if dest.Enabled {
				if dest.Created && !flags.dryRun {
					// PATCH: Newly-created playlists do not need duplicate
					// checks, and YouTube can take a moment to expose them
					// consistently across playlistItems endpoints.
					time.Sleep(2 * time.Second)
				}
				for i := range uploads {
					if flags.dryRun {
						uploads[i].PlaylistStatus = "would_add"
						added++
						continue
					}
					if !dest.Created {
						exists, err := playlistContainsVideo(c, dest.ID, uploads[i].VideoID)
						if err != nil {
							warnings = append(warnings, fmt.Sprintf("%s: checking playlist duplicate: %v", uploads[i].VideoID, err))
							uploads[i].PlaylistStatus = "error"
							if isQuotaExceeded(err) {
								break
							}
							continue
						}
						if exists {
							uploads[i].PlaylistStatus = "already_present"
							skipped++
							continue
						}
					}
					itemID, err := addVideoToPlaylist(c, dest.ID, uploads[i].VideoID)
					if err != nil {
						warnings = append(warnings, fmt.Sprintf("%s: adding to playlist: %v", uploads[i].VideoID, err))
						uploads[i].PlaylistStatus = "error"
						if isQuotaExceeded(err) {
							break
						}
						continue
					}
					uploads[i].PlaylistStatus = "added"
					uploads[i].PlaylistItemID = itemID
					added++
				}
			}

			out := recentSubscriptionUploadsOutput{
				Meta: recentSubscriptionUploadsMeta{
					Source:               "live",
					Since:                since,
					Cutoff:               cutoff.UTC().Format(time.RFC3339),
					SubscriptionsScanned: len(subscriptions),
					SubscriptionPages:    subscriptionPages,
					ChannelsChecked:      len(sources),
					ChannelBatches:       channelBatches,
					PerChannel:           perChannel,
					Concurrency:          concurrency,
					VideosFound:          len(uploads),
					PlaylistID:           dest.ID,
					PlaylistTitle:        dest.Title,
					PlaylistCreated:      dest.Created,
					AddedToPlaylist:      added,
					SkippedExisting:      skipped,
					DryRun:               flags.dryRun,
					Note:                 workflowNote(notificationScope, channelIDFile),
				},
				Results:  uploads,
				Warnings: warnings,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printRecentSubscriptionUploadsTable(cmd, out)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Look back duration (for example 24h, 2d, 1w)")
	cmd.Flags().StringVar(&subscriptionOrder, "subscription-order", "unread", "Subscription listing order: unread, relevance, alphabetical, or subscriptionOrderUnspecified")
	cmd.Flags().IntVar(&perChannel, "per-channel", 3, "Most recent uploads to inspect per subscribed channel (1-50)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 8, "Parallel uploads-playlist checks")
	cmd.Flags().IntVar(&maxSubscriptions, "max-subscriptions", 0, "Stop after this many subscriptions (0 = all)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit returned videos after sorting newest first (0 = no limit)")
	cmd.Flags().StringVar(&playlistID, "playlist-id", "", "Destination playlist ID for adding discovered videos")
	cmd.Flags().StringVar(&playlistTitle, "playlist-title", "", "Destination playlist title to find among your playlists")
	cmd.Flags().BoolVar(&createPlaylist, "create-playlist", false, "Create --playlist-title when it does not exist")
	cmd.Flags().StringVar(&privacyStatus, "privacy-status", "private", "Privacy for created playlists: private, unlisted, or public")
	cmd.Flags().StringVar(&notificationScope, "notification-scope", "", "Channel scope: subscriptions (all subscribed channels) or all (requires --channel-id-file from YouTube web bell-state export); default is all for workflow notifications, subscriptions otherwise")
	cmd.Flags().StringVar(&channelIDFile, "channel-id-file", "", "Optional file with one channel ID per line; notification mode defaults to "+defaultAllBellChannelIDFile()+" or YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE")

	return cmd
}

func resolveAllBellChannelIDFile(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if envPath := os.Getenv("YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE points to an unreadable channel file: %w", err)
		}
		return envPath, nil
	}
	defaultPath := defaultAllBellChannelIDFile()
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking default All-bell channel file %s: %w", defaultPath, err)
	}
	return "", fmt.Errorf("YouTube Data API does not expose the web bell setting (All/Personalized/None); create %s with one All-bell channel ID per line, set YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE, or pass --channel-id-file to use notification mode", defaultPath)
}

func defaultAllBellChannelIDFile() string {
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "youtube-pp-cli", "all-bell-channel-ids.txt")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "youtube-pp-cli", "all-bell-channel-ids.txt")
}

func loadChannelIDFilter(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --channel-id-file: %w", err)
	}
	filter := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			filter[fields[0]] = true
		}
	}
	if len(filter) == 0 {
		return nil, fmt.Errorf("--channel-id-file did not contain any channel IDs")
	}
	return filter, nil
}

func filterSubscribedChannels(channels []subscribedChannel, filter map[string]bool) []subscribedChannel {
	filtered := make([]subscribedChannel, 0, len(channels))
	for _, channel := range channels {
		if filter[channel.ChannelID] {
			filtered = append(filtered, channel)
		}
	}
	return filtered
}

func workflowNote(notificationScope, channelIDFile string) string {
	if notificationScope == "all" {
		return "Filtered to channel IDs supplied via --channel-id-file; YouTube Data API does not expose All/Personalized/None bell state directly."
	}
	return "YouTube Data API does not expose notification bell settings; results are recent uploads from all subscribed channels. Use --notification-scope all with --channel-id-file for a web-exported All-bell subset."
}

func fetchSubscribedChannels(c *client.Client, order string, maxItems int) ([]subscribedChannel, int, error) {
	params := map[string]string{
		"part":       "snippet,contentDetails",
		"mine":       "true",
		"maxResults": "50",
	}
	if order != "" {
		params["order"] = order
	}
	items, pages, err := fetchYouTubeItems(c, "/youtube/v3/subscriptions", params, 0, maxItems)
	if err != nil {
		return nil, pages, err
	}
	channels := make([]subscribedChannel, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		var item youtubeSubscriptionItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, pages, err
		}
		channelID := item.Snippet.ResourceID.ChannelID
		if channelID == "" || seen[channelID] {
			continue
		}
		seen[channelID] = true
		channels = append(channels, subscribedChannel{
			ChannelID:    channelID,
			ChannelTitle: item.Snippet.Title,
			NewItemCount: item.ContentDetails.NewItemCount,
		})
	}
	return channels, pages, nil
}

func fetchChannelUploadsSources(c *client.Client, subscriptions []subscribedChannel) ([]channelUploadsSource, int, error) {
	const batchSize = 50
	byID := map[string]channelUploadsSource{}
	batches := 0
	for start := 0; start < len(subscriptions); start += batchSize {
		end := start + batchSize
		if end > len(subscriptions) {
			end = len(subscriptions)
		}
		ids := make([]string, 0, end-start)
		for _, sub := range subscriptions[start:end] {
			ids = append(ids, sub.ChannelID)
		}
		rawItems, _, err := fetchYouTubeItems(c, "/youtube/v3/channels", map[string]string{
			"part":       "snippet,contentDetails",
			"id":         strings.Join(ids, ","),
			"maxResults": "50",
		}, 1, 0)
		if err != nil {
			return nil, batches, err
		}
		batches++
		for _, raw := range rawItems {
			var item youtubeChannelItem
			if err := json.Unmarshal(raw, &item); err != nil {
				return nil, batches, err
			}
			uploads := item.ContentDetails.RelatedPlaylists.Uploads
			if item.ID == "" || uploads == "" {
				continue
			}
			byID[item.ID] = channelUploadsSource{
				ChannelID:         item.ID,
				ChannelTitle:      item.Snippet.Title,
				UploadsPlaylistID: uploads,
			}
		}
	}

	sources := make([]channelUploadsSource, 0, len(byID))
	for _, sub := range subscriptions {
		source, ok := byID[sub.ChannelID]
		if !ok {
			continue
		}
		if source.ChannelTitle == "" {
			source.ChannelTitle = sub.ChannelTitle
		}
		sources = append(sources, source)
	}
	return sources, batches, nil
}

func fetchRecentUploadsFromSources(ctx context.Context, c *client.Client, sources []channelUploadsSource, cutoff time.Time, perChannel, concurrency int) ([]recentSubscriptionUpload, []string) {
	results, errs := cliutil.FanoutRun(ctx, sources,
		func(source channelUploadsSource) string { return source.ChannelTitle },
		func(_ context.Context, source channelUploadsSource) ([]recentSubscriptionUpload, error) {
			return fetchRecentUploadsForSource(c, source, cutoff, perChannel)
		},
		cliutil.WithConcurrency(concurrency),
	)

	uploads := make([]recentSubscriptionUpload, 0)
	for _, result := range results {
		uploads = append(uploads, result.Value...)
	}
	warnings := make([]string, 0, len(errs))
	for _, err := range errs {
		warnings = append(warnings, fmt.Sprintf("%s: %v", err.Source, err.Err))
	}
	return uploads, warnings
}

func fetchRecentUploadsForSource(c *client.Client, source channelUploadsSource, cutoff time.Time, perChannel int) ([]recentSubscriptionUpload, error) {
	rawItems, _, err := fetchYouTubeItems(c, "/youtube/v3/playlistItems", map[string]string{
		"part":       "snippet,contentDetails",
		"playlistId": source.UploadsPlaylistID,
		"maxResults": strconv.Itoa(perChannel),
	}, 1, perChannel)
	if err != nil {
		return nil, err
	}

	uploads := make([]recentSubscriptionUpload, 0, len(rawItems))
	for _, raw := range rawItems {
		var item youtubePlaylistItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		publishedAt := item.ContentDetails.VideoPublishedAt
		if publishedAt == "" {
			publishedAt = item.Snippet.PublishedAt
		}
		published, ok := parseYouTubeTime(publishedAt)
		if !ok || published.Before(cutoff) {
			continue
		}
		videoID := item.ContentDetails.VideoID
		if videoID == "" {
			videoID = item.Snippet.ResourceID.VideoID
		}
		if videoID == "" {
			continue
		}
		channelTitle := item.Snippet.VideoOwnerChannelTitle
		if channelTitle == "" {
			channelTitle = source.ChannelTitle
		}
		channelID := item.Snippet.VideoOwnerChannelID
		if channelID == "" {
			channelID = source.ChannelID
		}
		uploads = append(uploads, recentSubscriptionUpload{
			VideoID:           videoID,
			Title:             item.Snippet.Title,
			ChannelID:         channelID,
			ChannelTitle:      channelTitle,
			PublishedAt:       published.UTC().Format(time.RFC3339),
			URL:               "https://www.youtube.com/watch?v=" + videoID,
			ThumbnailURL:      bestThumbnailURL(item.Snippet.Thumbnails),
			UploadsPlaylistID: source.UploadsPlaylistID,
		})
	}
	return uploads, nil
}

func fetchYouTubeItems(c *client.Client, path string, params map[string]string, maxPages, itemLimit int) ([]json.RawMessage, int, error) {
	clean := map[string]string{}
	for k, v := range params {
		if v != "" && v != "0" && v != "false" {
			clean[k] = v
		}
	}

	items := make([]json.RawMessage, 0)
	pages := 0
	lastCursor := ""
	for {
		data, err := c.Get(path, clean)
		if err != nil {
			return nil, pages, err
		}
		var envelope youtubeListEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, pages, fmt.Errorf("parsing %s response: %w", path, err)
		}
		pages++
		for _, item := range envelope.Items {
			if itemLimit > 0 && len(items) >= itemLimit {
				return items, pages, nil
			}
			items = append(items, item)
		}
		if envelope.NextPageToken == "" {
			break
		}
		if envelope.NextPageToken == lastCursor {
			return nil, pages, fmt.Errorf("pagination cursor did not advance for %s: nextPageToken %q repeated", path, envelope.NextPageToken)
		}
		lastCursor = envelope.NextPageToken
		if maxPages > 0 && pages >= maxPages {
			break
		}
		clean["pageToken"] = envelope.NextPageToken
	}
	return items, pages, nil
}

func resolveDestinationPlaylist(c *client.Client, playlistID, playlistTitle string, create bool, privacyStatus string, dryRun bool) (destinationPlaylist, error) {
	switch {
	case playlistID != "":
		return destinationPlaylist{Enabled: true, ID: playlistID}, nil
	case playlistTitle == "":
		return destinationPlaylist{}, nil
	}

	playlists, _, err := fetchYouTubeItems(c, "/youtube/v3/playlists", map[string]string{
		"part":       "snippet",
		"mine":       "true",
		"maxResults": "50",
	}, 100, 0)
	if err != nil {
		return destinationPlaylist{}, err
	}
	for _, raw := range playlists {
		var playlist youtubePlaylist
		if err := json.Unmarshal(raw, &playlist); err != nil {
			return destinationPlaylist{}, err
		}
		if strings.EqualFold(strings.TrimSpace(playlist.Snippet.Title), strings.TrimSpace(playlistTitle)) {
			return destinationPlaylist{Enabled: true, ID: playlist.ID, Title: playlist.Snippet.Title}, nil
		}
	}

	if !create {
		return destinationPlaylist{}, fmt.Errorf("playlist %q not found; pass --create-playlist or use --playlist-id", playlistTitle)
	}
	if dryRun {
		return destinationPlaylist{Enabled: true, Title: playlistTitle, Created: true}, nil
	}

	body := map[string]any{
		"snippet": map[string]any{
			"title":       playlistTitle,
			"description": "Created by youtube-pp-cli recent-subscription-uploads workflow.",
		},
		"status": map[string]any{
			"privacyStatus": privacyStatus,
		},
	}
	data, _, err := c.PostWithParams("/youtube/v3/playlists", map[string]string{"part": "snippet,status"}, body)
	if err != nil {
		return destinationPlaylist{}, err
	}
	var playlist youtubePlaylist
	if err := json.Unmarshal(data, &playlist); err != nil {
		return destinationPlaylist{}, err
	}
	return destinationPlaylist{Enabled: true, ID: playlist.ID, Title: playlist.Snippet.Title, Created: true}, nil
}

func playlistContainsVideo(c *client.Client, playlistID, videoID string) (bool, error) {
	items, _, err := fetchYouTubeItems(c, "/youtube/v3/playlistItems", map[string]string{
		"part":       "id",
		"playlistId": playlistID,
		"videoId":    videoID,
		"maxResults": "1",
	}, 1, 1)
	if err != nil {
		return false, err
	}
	return len(items) > 0, nil
}

func addVideoToPlaylist(c *client.Client, playlistID, videoID string) (string, error) {
	body := map[string]any{
		"snippet": map[string]any{
			"playlistId": playlistID,
			"resourceId": map[string]any{
				"kind":    "youtube#video",
				"videoId": videoID,
			},
		},
	}
	data, _, err := c.PostWithParams("/youtube/v3/playlistItems", map[string]string{"part": "snippet"}, body)
	if err != nil {
		return "", err
	}
	var inserted youtubePlaylistItem
	if err := json.Unmarshal(data, &inserted); err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func isQuotaExceeded(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "quotaExceeded")
}

func parseYouTubeTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return t, true
	}
	t, err = time.Parse("2006-01-02T15:04:05.000Z", raw)
	if err == nil {
		return t, true
	}
	return time.Time{}, false
}

func bestThumbnailURL(thumbnails map[string]youtubeThumbnail) string {
	for _, key := range []string{"maxres", "standard", "high", "medium", "default"} {
		if thumb, ok := thumbnails[key]; ok && thumb.URL != "" {
			return thumb.URL
		}
	}
	return ""
}

func printRecentSubscriptionUploadsTable(cmd *cobra.Command, out recentSubscriptionUploadsOutput) {
	fmt.Fprintf(cmd.ErrOrStderr(), "%d recent uploads from %d subscribed channels\n", out.Meta.VideosFound, out.Meta.SubscriptionsScanned)
	for _, warning := range out.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PUBLISHED\tCHANNEL\tTITLE\tSTATUS\tURL")
	for _, item := range out.Results {
		status := item.PlaylistStatus
		if status == "" {
			status = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			item.PublishedAt,
			truncate(item.ChannelTitle, 32),
			truncate(item.Title, 56),
			status,
			item.URL,
		)
	}
	_ = tw.Flush()
}
