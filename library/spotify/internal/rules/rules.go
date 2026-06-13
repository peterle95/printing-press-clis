package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Playlists       map[string]PlaylistRule `yaml:"playlists" json:"playlists"`
	Fallback        FallbackRule            `yaml:"fallback" json:"fallback"`
	ManualOverrides ManualOverrides         `yaml:"manual_overrides" json:"manual_overrides"`
}

type PlaylistRule struct {
	Name            string    `yaml:"name" json:"name"`
	CreateIfMissing bool      `yaml:"create_if_missing" json:"create_if_missing"`
	Public          bool      `yaml:"public" json:"public"`
	Priority        int       `yaml:"priority" json:"priority"`
	Match           MatchRule `yaml:"match" json:"match"`
}

type MatchRule struct {
	AnyArtistGenreContains []string `yaml:"any_artist_genre_contains" json:"any_artist_genre_contains"`
}

type FallbackRule struct {
	Name            string `yaml:"name" json:"name"`
	CreateIfMissing bool   `yaml:"create_if_missing" json:"create_if_missing"`
	Public          bool   `yaml:"public" json:"public"`
}

type ManualOverrides struct {
	Tracks  map[string]string `yaml:"tracks" json:"tracks"`
	Artists map[string]string `yaml:"artists" json:"artists"`
}

type ResolvedRule struct {
	Key  string
	Rule PlaylistRule
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	normalizeFile(&f)
	return &f, nil
}

func Init(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, DefaultYAML(), 0o600)
}

func Validate(f *File) []error {
	var errs []error
	if f == nil {
		return []error{fmt.Errorf("rules file is empty")}
	}
	if len(f.Playlists) == 0 {
		errs = append(errs, fmt.Errorf("rules file must contain at least one playlist rule"))
	}
	names := map[string]string{}
	for key, rule := range f.Playlists {
		if strings.TrimSpace(rule.Name) == "" {
			errs = append(errs, fmt.Errorf("playlist rule %q is missing name", key))
		}
		if priorKey, ok := names[strings.ToLower(rule.Name)]; ok {
			errs = append(errs, fmt.Errorf("playlist rules %q and %q use the same playlist name %q", priorKey, key, rule.Name))
		}
		names[strings.ToLower(rule.Name)] = key
		for _, m := range rule.Match.AnyArtistGenreContains {
			if NormalizeGenre(m) == "" {
				errs = append(errs, fmt.Errorf("playlist rule %q contains an empty genre matcher", key))
			}
		}
	}
	if strings.TrimSpace(f.Fallback.Name) == "" {
		errs = append(errs, fmt.Errorf("fallback.name is required"))
	}
	return errs
}

func (f *File) SortedRules() []ResolvedRule {
	out := make([]ResolvedRule, 0, len(f.Playlists))
	for key, rule := range f.Playlists {
		out = append(out, ResolvedRule{Key: key, Rule: rule})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rule.Priority == out[j].Rule.Priority {
			return out[i].Key < out[j].Key
		}
		return out[i].Rule.Priority < out[j].Rule.Priority
	})
	return out
}

func NormalizeGenre(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = punctuation.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func DefaultYAML() []byte {
	return bytes.TrimSpace([]byte(defaultYAML + "\n"))
}

var punctuation = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeFile(f *File) {
	if f.Playlists == nil {
		f.Playlists = map[string]PlaylistRule{}
	}
	if f.ManualOverrides.Tracks == nil {
		f.ManualOverrides.Tracks = map[string]string{}
	}
	if f.ManualOverrides.Artists == nil {
		f.ManualOverrides.Artists = map[string]string{}
	}
	if f.Fallback.Name == "" {
		f.Fallback.Name = "Needs Review"
		f.Fallback.CreateIfMissing = true
	}
}

const defaultYAML = `
playlists:
  techno:
    name: "Techno"
    create_if_missing: true
    public: false
    priority: 10
    match:
      any_artist_genre_contains:
        - techno
        - minimal techno
        - detroit techno
        - acid techno

  house:
    name: "House"
    create_if_missing: true
    public: false
    priority: 20
    match:
      any_artist_genre_contains:
        - house
        - deep house
        - tech house
        - progressive house

  rock:
    name: "Rock"
    create_if_missing: true
    public: false
    priority: 30
    match:
      any_artist_genre_contains:
        - rock
        - alternative rock
        - indie rock
        - punk

  classical:
    name: "Classical"
    create_if_missing: true
    public: false
    priority: 40
    match:
      any_artist_genre_contains:
        - classical
        - orchestra
        - piano
        - romantic era
        - baroque

  rap:
    name: "Rap / Hip-Hop"
    create_if_missing: true
    public: false
    priority: 50
    match:
      any_artist_genre_contains:
        - hip hop
        - rap
        - trap
        - drill

fallback:
  name: "Needs Review"
  create_if_missing: true
  public: false

manual_overrides:
  tracks:
    "spotify:track:...": "Techno"
  artists:
    "spotify:artist:...": "House"
`
