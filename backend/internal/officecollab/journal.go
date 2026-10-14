package officecollab

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

// Recovery sidecars live in FileBrowser's private cache, not a document database.
// The ordinary Office file remains the durable, directly usable document.
type journal struct {
	ID       string            `json:"id"`
	Path     string            `json:"path"`
	Digest   string            `json:"digest"`
	Revision int64             `json:"revision"`
	Saved    int64             `json:"saved"`
	Sequence int64             `json:"sequence"`
	Events   []Event           `json:"events"`
	Batches  map[string]int64  `json:"batches"`
	Owners   map[string]string `json:"owners"`
}

func (m *Manager) SetJournalDir(dir string) { m.mu.Lock(); defer m.mu.Unlock(); m.journalDir = dir }
func (m *Manager) journalPath(path string) string {
	return filepath.Join(m.journalDir, fingerprint([]byte(path))+".json")
}
func (m *Manager) persist(r *room) error {
	if m.journalDir == "" {
		return nil
	}
	if err := os.MkdirAll(m.journalDir, 0700); err != nil {
		return err
	}
	base := m.journalPath(r.path) + "." + r.id + ".base"
	if _, err := os.Stat(base); os.IsNotExist(err) {
		if err = os.WriteFile(base, r.base, 0600); err != nil {
			return err
		}
	}
	events := []Event{}
	for _, e := range r.events {
		if e.Type == "changes" {
			events = append(events, e)
		}
	}
	data, err := json.Marshal(journal{ID: r.id, Path: r.path, Digest: r.digest, Revision: r.revision, Saved: r.saved, Sequence: r.sequence, Events: events, Batches: r.batches, Owners: r.owners})
	if err != nil {
		return err
	}
	target := m.journalPath(r.path)
	tmp := target + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
func (m *Manager) recover(path string) (*room, error) {
	if m.journalDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(m.journalPath(path))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j journal
	if err = json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("session recovery journal is invalid: %w", err)
	}
	if j.Path != path {
		return nil, fmt.Errorf("session recovery path does not match")
	}
	if _, err = uuid.Parse(j.ID); err != nil {
		return nil, fmt.Errorf("session recovery identifier is invalid")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if fingerprint(current) != j.Digest {
		return nil, fmt.Errorf("file changed outside this session; preserve its recovery journal before reopening")
	}
	base, err := os.ReadFile(m.journalPath(path) + "." + j.ID + ".base")
	if err != nil {
		return nil, err
	}
	bytes := 0
	for _, e := range j.Events {
		for _, c := range e.Changes {
			bytes += len(c)
		}
	}
	return &room{id: j.ID, path: path, digest: j.Digest, base: base, revision: j.Revision, saved: j.Saved, sequence: j.Sequence, events: j.Events, batches: j.Batches, owners: j.Owners, bytes: bytes, clients: map[string]Participant{}}, nil
}
func (m *Manager) removeJournal(path string) {
	if m.journalDir != "" {
		var j journal
		if data, err := os.ReadFile(m.journalPath(path)); err == nil && json.Unmarshal(data, &j) == nil {
			if _, err = uuid.Parse(j.ID); err != nil {
				return
			}
			os.Remove(m.journalPath(path) + "." + j.ID + ".base")
		}
		os.Remove(m.journalPath(path))
	}
}
