// Package csvimport parses user-exported CSV data without assuming one header layout.
package csvimport

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Set struct {
	WorkoutTitle, StartText, EndText, ExerciseTitle, SupersetID, SetType, Notes string
	SetIndex                                                                    int
	WeightKG, Reps, DurationSeconds, DistanceMeters, RPE                        *float64
	Raw                                                                         map[string]string
}
type Result struct {
	FileHash string
	RowsRead int
	Sets     []Set
	Warnings []string
	Format   string
}

func HeaderKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var fields = map[string][]string{"workouttitle": {"workouttitle", "title", "workoutname"}, "start": {"workoutstart", "starttime", "start", "startdate"}, "end": {"workoutend", "endtime", "end", "enddate"}, "exercise": {"exercisetitle", "exercisename", "exercise"}, "superset": {"superset", "supersetid"}, "setindex": {"setindex", "setnumber", "set"}, "settype": {"settype", "type"}, "weight": {"weightkg", "weight", "kg"}, "reps": {"reps", "repetitions", "repscount"}, "duration": {"durationseconds", "duration", "seconds"}, "distance": {"distancemeters", "distance", "meters"}, "rpe": {"rpe"}, "notes": {"notes", "note"}}

func ParseFile(path string, maxBytes int64) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	if st, e := f.Stat(); e == nil && st.Size() > maxBytes {
		return Result{}, fmt.Errorf("CSV exceeds %d-byte limit", maxBytes)
	}
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(f, h))
	if err != nil {
		return Result{}, err
	}
	result, err := Parse(strings.NewReader(string(data)))
	result.FileHash = hex.EncodeToString(h.Sum(nil))
	return result, err
}
func Parse(r io.Reader) (Result, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return Result{}, fmt.Errorf("read CSV header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		k := HeaderKey(h)
		for canonical, aliases := range fields {
			for _, a := range aliases {
				if k == a {
					idx[canonical] = i
				}
			}
		}
	}
	if idx["exercise"] == 0 && HeaderKey(header[0]) != "exercise" && HeaderKey(header[0]) != "exercisetitle" {
		return Result{}, fmt.Errorf("unsupported CSV: no exercise title column")
	}
	if _, ok := idx["workouttitle"]; !ok {
		return Result{}, fmt.Errorf("unsupported CSV: no workout title column")
	}
	if _, ok := idx["start"]; !ok {
		return Result{}, fmt.Errorf("unsupported CSV: no workout start column")
	}
	result := Result{Format: "hevy-dynamic-csv"}
	for rowNo := 2; ; rowNo++ {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("read row %d: %w", rowNo, err)
		}
		result.RowsRead++
		get := func(k string) string {
			i, ok := idx[k]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		raw := map[string]string{}
		for i, h := range header {
			if i < len(row) {
				raw[h] = row[i]
			}
		}
		s := Set{WorkoutTitle: get("workouttitle"), StartText: get("start"), EndText: get("end"), ExerciseTitle: get("exercise"), SupersetID: get("superset"), SetType: get("settype"), Notes: get("notes"), Raw: raw}
		if s.ExerciseTitle == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("row %d skipped: empty exercise", rowNo))
			continue
		}
		var e error
		if s.SetIndex, e = parseInt(get("setindex")); e != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("row %d: invalid set index", rowNo))
		}
		s.WeightKG, _ = parseDecimal(get("weight"))
		s.Reps, _ = parseDecimal(get("reps"))
		s.DurationSeconds, _ = parseDecimal(get("duration"))
		s.DistanceMeters, _ = parseDecimal(get("distance"))
		s.RPE, _ = parseDecimal(get("rpe"))
		result.Sets = append(result.Sets, s)
	}
	return result, nil
}
func parseInt(s string) (int, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.TrimSpace(s))
}
func parseDecimal(s string) (*float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return nil, nil
	}
	v, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return nil, e
	}
	return &v, nil
}
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05 -0700", "2006-01-02 15:04:05", "02/01/2006 15:04:05", "01/02/2006 15:04:05"}
	for _, l := range layouts {
		if t, e := time.Parse(l, s); e == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", s)
}
func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		io.WriteString(h, strings.ToLower(strings.TrimSpace(p)))
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
}
