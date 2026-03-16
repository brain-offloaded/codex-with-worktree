package codexlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WorktreeAggregate struct {
	SessionCount   int
	TurnCount      int
	LastActivityAt time.Time
}

type SessionAggregate struct {
	SessionID      string
	CWD            string
	FirstSeenAt    time.Time
	LastActivityAt time.Time
	TurnCount      int
}

type Aggregate struct {
	ByCWD     map[string]WorktreeAggregate
	BySession map[string]SessionAggregate
}

type envelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	ID  string `json:"id"`
	CWD string `json:"cwd"`
}

func Scan(codexHome string) (Aggregate, error) {
	sessionsRoot := filepath.Join(codexHome, "sessions")
	if st, err := os.Stat(sessionsRoot); err != nil || !st.IsDir() {
		return Aggregate{ByCWD: map[string]WorktreeAggregate{}, BySession: map[string]SessionAggregate{}}, nil
	}

	result := Aggregate{
		ByCWD:     map[string]WorktreeAggregate{},
		BySession: map[string]SessionAggregate{},
	}
	err := filepath.WalkDir(sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		return scanFile(path, &result)
	})
	return result, err
}

func scanFile(path string, agg *Aggregate) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var sessionID string
	var cwd string
	for scanner.Scan() {
		line := scanner.Bytes()
		var env envelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, env.Timestamp)
		if err != nil {
			continue
		}
		if env.Type == "session_meta" {
			var meta sessionMetaPayload
			if err := json.Unmarshal(env.Payload, &meta); err == nil && meta.ID != "" {
				sessionID = meta.ID
				cwd = meta.CWD
				upsertSession(agg, sessionID, cwd, ts, false)
			}
			continue
		}
		if sessionID == "" {
			continue
		}
		upsertSession(agg, sessionID, cwd, ts, true)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func upsertSession(agg *Aggregate, sessionID, cwd string, ts time.Time, countTurn bool) {
	s := agg.BySession[sessionID]
	if s.SessionID == "" {
		s.SessionID = sessionID
		s.CWD = cwd
		s.FirstSeenAt = ts
	}
	if s.CWD == "" {
		s.CWD = cwd
	}
	if s.FirstSeenAt.IsZero() || ts.Before(s.FirstSeenAt) {
		s.FirstSeenAt = ts
	}
	if ts.After(s.LastActivityAt) {
		s.LastActivityAt = ts
	}
	if countTurn {
		s.TurnCount++
	}
	agg.BySession[sessionID] = s

	if s.CWD == "" {
		return
	}
	w := agg.ByCWD[s.CWD]
	seenNew := w.SessionCount == 0
	if seenNew {
		w.SessionCount = 1
	} else {
		w.SessionCount = countSessionsForCWD(agg.BySession, s.CWD)
	}
	w.TurnCount = totalTurnsForCWD(agg.BySession, s.CWD)
	if ts.After(w.LastActivityAt) {
		w.LastActivityAt = ts
	}
	agg.ByCWD[s.CWD] = w
}

func countSessionsForCWD(sessions map[string]SessionAggregate, cwd string) int {
	count := 0
	for _, s := range sessions {
		if s.CWD == cwd {
			count++
		}
	}
	return count
}

func totalTurnsForCWD(sessions map[string]SessionAggregate, cwd string) int {
	total := 0
	for _, s := range sessions {
		if s.CWD == cwd {
			total += s.TurnCount
		}
	}
	return total
}
