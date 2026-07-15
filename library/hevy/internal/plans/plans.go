// Package plans owns the portable YAML/JSON workout-plan representation.
package plans

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type Plan struct {
	Version     int       `yaml:"version" json:"version"`
	Name        string    `yaml:"name" json:"name"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Tags        []string  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Routines    []Routine `yaml:"routines" json:"routines"`
}
type Routine struct {
	Name      string     `yaml:"name" json:"name"`
	Notes     string     `yaml:"notes,omitempty" json:"notes,omitempty"`
	Exercises []Exercise `yaml:"exercises" json:"exercises"`
}
type Exercise struct {
	Name        string `yaml:"name" json:"name"`
	Notes       string `yaml:"notes,omitempty" json:"notes,omitempty"`
	Order       *int   `yaml:"order,omitempty" json:"order,omitempty"`
	RestSeconds *int   `yaml:"rest_seconds,omitempty" json:"rest_seconds,omitempty"`
	Sets        []Set  `yaml:"sets" json:"sets"`
}
type Reps struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}
type Set struct {
	Type                  string   `yaml:"type" json:"type"`
	TargetReps            *Reps    `yaml:"target_reps,omitempty" json:"target_reps,omitempty"`
	TargetWeightKG        *float64 `yaml:"target_weight_kg,omitempty" json:"target_weight_kg,omitempty"`
	TargetDurationSeconds *int     `yaml:"target_duration_seconds,omitempty" json:"target_duration_seconds,omitempty"`
	TargetDistanceMeters  *float64 `yaml:"target_distance_meters,omitempty" json:"target_distance_meters,omitempty"`
	Notes                 string   `yaml:"notes,omitempty" json:"notes,omitempty"`
}

func Decode(data []byte, source string) (Plan, error) {
	var p Plan
	if strings.EqualFold(filepath.Ext(source), ".json") {
		if err := json.Unmarshal(data, &p); err != nil {
			return p, fmt.Errorf("parse JSON plan: %w", err)
		}
	} else if err := yaml.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parse YAML plan: %w", err)
	}
	return p, Validate(p)
}
func Read(path string) (Plan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}
	return Decode(b, path)
}

func Validate(p Plan) error {
	var errs []string
	if p.Version != CurrentVersion {
		errs = append(errs, fmt.Sprintf("version: expected %d, got %d", CurrentVersion, p.Version))
	}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "name: must not be empty")
	}
	seen := map[string]bool{}
	for ri, r := range p.Routines {
		path := fmt.Sprintf("routines[%d]", ri)
		name := strings.TrimSpace(r.Name)
		if name == "" {
			errs = append(errs, path+".name: must not be empty")
		} else if seen[strings.ToLower(name)] {
			errs = append(errs, path+".name: duplicates another routine")
		}
		seen[strings.ToLower(name)] = true
		for ei, e := range r.Exercises {
			ep := fmt.Sprintf("%s.exercises[%d]", path, ei)
			if strings.TrimSpace(e.Name) == "" {
				errs = append(errs, ep+".name: must not be empty")
			}
			if e.Order != nil && *e.Order < 0 {
				errs = append(errs, ep+".order: must not be negative")
			}
			if e.RestSeconds != nil && *e.RestSeconds < 0 {
				errs = append(errs, ep+".rest_seconds: must not be negative")
			}
			for si, s := range e.Sets {
				sp := fmt.Sprintf("%s.sets[%d]", ep, si)
				if s.TargetReps != nil {
					if s.TargetReps.Min < 0 || s.TargetReps.Max < 0 {
						errs = append(errs, sp+".target_reps: must not be negative")
					}
					if s.TargetReps.Min > s.TargetReps.Max {
						errs = append(errs, sp+".target_reps.min: must not exceed max")
					}
				}
				if s.TargetWeightKG != nil && *s.TargetWeightKG < 0 {
					errs = append(errs, sp+".target_weight_kg: must not be negative")
				}
				if s.TargetDurationSeconds != nil && *s.TargetDurationSeconds < 0 {
					errs = append(errs, sp+".target_duration_seconds: must not be negative")
				}
				if s.TargetDistanceMeters != nil && *s.TargetDistanceMeters < 0 {
					errs = append(errs, sp+".target_distance_meters: must not be negative")
				}
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("invalid plan: %s", strings.Join(errs, "; "))
	}
	return nil
}

var unsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

func Filename(name string) (string, error) {
	n := strings.Trim(strings.ToLower(name), " .")
	n = unsafe.ReplaceAllString(n, "-")
	n = strings.Trim(n, "-")
	if n == "" || n == "." || n == ".." {
		return "", fmt.Errorf("invalid plan name")
	}
	return n, nil
}
func Save(dir string, p Plan, format string, force bool) (string, error) {
	if err := Validate(p); err != nil {
		return "", err
	}
	f, err := Filename(p.Name)
	if err != nil {
		return "", err
	}
	ext := ".yaml"
	var b []byte
	if format == "json" {
		ext = ".json"
		b, err = json.MarshalIndent(p, "", "  ")
		b = append(b, '\n')
	} else {
		b, err = yaml.Marshal(p)
	}
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, f+ext)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("plan already exists: %s (use --force)", path)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".plan-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return path, os.Rename(tmpName, path)
}
func Find(dir, name string) (Plan, string, error) {
	f, err := Filename(name)
	if err != nil {
		return Plan{}, "", err
	}
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		p := filepath.Join(dir, f+ext)
		if _, err := os.Stat(p); err == nil {
			x, err := Read(p)
			return x, p, err
		}
	}
	return Plan{}, "", fmt.Errorf("plan not found: %q", name)
}
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".json")) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}
