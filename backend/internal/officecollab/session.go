// Package officecollab coordinates transient native-editor sessions. Ordinary
// Office files remain the durable document storage; sessions never use a database.
package officecollab

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrConflict = errors.New("collaboration state changed; synchronize before saving")
var ErrSession = errors.New("document session expired; keep this tab open and download your draft before reopening")
var ErrRestored = errors.New("a previous version was restored; reload the current document, preserving any unsaved draft")

type Participant struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Edit bool      `json:"edit"`
	Seen time.Time `json:"-"`
}
type Event struct {
	Seq      int64             `json:"seq"`
	Revision int64             `json:"revision"`
	Type     string            `json:"type"`
	Client   string            `json:"client"`
	Author   string            `json:"author,omitempty"`
	Changes  []json.RawMessage `json:"changes,omitempty"`
	Extra    json.RawMessage   `json:"extra,omitempty"`
	Cursor   string            `json:"cursor,omitempty"`
	Blocks   []json.RawMessage `json:"blocks,omitempty"`
}
type State struct {
	Room         string        `json:"room"`
	Client       string        `json:"client"`
	Revision     int64         `json:"revision"`
	Saved        int64         `json:"saved"`
	Sequence     int64         `json:"sequence"`
	Events       []Event       `json:"events"`
	Participants []Participant `json:"participants"`
	Lease        string        `json:"lease,omitempty"`
}
type room struct {
	id, path, digest          string
	base                      []byte
	revision, saved, sequence int64
	events                    []Event
	clients                   map[string]Participant
	batches                   map[string]int64
	lease, leaseOwner         string
	leaseUntil                time.Time
	bytes                     int
	locks                     []editLock
	owners                    map[string]string
}

type editLock struct {
	Client string
	Block  json.RawMessage
}

func lockConflict(a, b json.RawMessage) bool {
	var x, y map[string]interface{}
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return string(a) == string(b)
	}
	delete(x, "guid")
	delete(y, "guid")
	if x["sheetId"] != nil && y["sheetId"] != nil {
		if x["sheetId"] != y["sheetId"] {
			return false
		}
		if x["type"] == float64(3) || y["type"] == float64(3) {
			return true
		}
		xr, xok := x["rangeOrObjectId"].(map[string]interface{})
		yr, yok := y["rangeOrObjectId"].(map[string]interface{})
		if xok && yok {
			num := func(m map[string]interface{}, k string) float64 { v, _ := m[k].(float64); return v }
			return num(xr, "c1") <= num(yr, "c2") && num(yr, "c1") <= num(xr, "c2") && num(xr, "r1") <= num(yr, "r2") && num(yr, "r1") <= num(xr, "r2")
		}
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return string(xb) == string(yb)
}
func (m *Manager) Locks(path, roomID, client string, blocks []json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if !r.clients[client].Edit {
		return os.ErrPermission
	}
	if len(blocks) > 1024 {
		return fmt.Errorf("too many editing locks")
	}
	for _, block := range blocks {
		for _, lock := range r.locks {
			if lock.Client != client && lockConflict(block, lock.Block) {
				return ErrConflict
			}
		}
	}
	for _, block := range blocks {
		r.locks = append(r.locks, editLock{Client: client, Block: block})
	}
	r.sequence++
	r.events = append(r.events, Event{Seq: r.sequence, Revision: r.revision, Type: "locks", Client: client, Blocks: blocks})
	return nil
}
func (m *Manager) releaseLocks(r *room, client string) {
	blocks := []json.RawMessage{}
	locks := r.locks[:0]
	for _, lock := range r.locks {
		if lock.Client == client {
			blocks = append(blocks, lock.Block)
		} else {
			locks = append(locks, lock)
		}
	}
	r.locks = locks
	if len(blocks) > 0 {
		r.sequence++
		r.events = append(r.events, Event{Seq: r.sequence, Revision: r.revision, Type: "unlocks", Client: client, Blocks: blocks})
	}
}

type Manager struct {
	mu         sync.Mutex
	rooms      map[string]*room
	journalDir string
}

func New() *Manager                  { return &Manager{rooms: make(map[string]*room)} }
func fingerprint(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func (m *Manager) Join(path, name string, edit bool) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, old := range m.rooms {
		m.expire(old)
		if len(old.clients) == 0 && old.saved == old.revision {
			m.removeJournal(key)
			delete(m.rooms, key)
		}
	}
	r := m.rooms[path]
	if r == nil {
		var err error
		r, err = m.recover(path)
		if err != nil {
			return State{}, err
		}
		if r != nil {
			m.rooms[path] = r
		}
	}
	if r != nil {
		m.expire(r)
	}
	if r == nil || (len(r.clients) == 0 && r.saved == r.revision) {
		if r != nil {
			m.removeJournal(path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return State{}, err
		}
		if len(data) > 100<<20 {
			return State{}, fmt.Errorf("live collaboration supports documents up to 100 MiB")
		}
		if len(m.rooms) >= 128 && r == nil {
			return State{}, fmt.Errorf("too many active document sessions")
		}
		r = &room{id: uuid.NewString(), path: path, base: data, digest: fingerprint(data), clients: map[string]Participant{}, batches: map[string]int64{}}
		m.rooms[path] = r
	}
	id := uuid.NewString()
	if len(r.clients) >= 64 {
		return State{}, fmt.Errorf("too many devices in this document session")
	}
	r.clients[id] = Participant{ID: id, Name: name, Edit: edit, Seen: time.Now()}
	return m.state(r, id, 0), nil
}

func (m *Manager) Active(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rooms[path]
	if r == nil {
		return false
	}
	m.expire(r)
	return len(r.clients) > 0
}

func (m *Manager) BindOwner(path, roomID, client, principal string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if r.owners == nil {
		r.owners = map[string]string{}
	}
	r.owners[client] = principal
	return m.persist(r)
}
func (m *Manager) Resume(path, roomID, client, principal, name string, edit bool, after int64) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rooms[path]
	if r == nil {
		var err error
		r, err = m.recover(path)
		if err != nil {
			return State{}, err
		}
		if r != nil {
			m.rooms[path] = r
		}
	}
	if r == nil || r.id != roomID {
		return State{}, ErrSession
	}
	if r.owners[client] != principal {
		return State{}, os.ErrPermission
	}
	r.clients[client] = Participant{ID: client, Name: name, Edit: edit, Seen: time.Now()}
	return m.state(r, client, after), nil
}
func (m *Manager) expire(r *room) {
	for id, p := range r.clients {
		if time.Since(p.Seen) > 45*time.Second {
			m.releaseLocks(r, id)
			delete(r.clients, id)
		}
	}
	if time.Now().After(r.leaseUntil) {
		r.lease = ""
		r.leaseOwner = ""
	}
}
func (m *Manager) get(path, roomID, client string) (*room, error) {
	r := m.rooms[path]
	if r == nil || r.id != roomID {
		return nil, ErrSession
	}
	m.expire(r)
	p, ok := r.clients[client]
	if !ok {
		return nil, ErrSession
	}
	p.Seen = time.Now()
	r.clients[client] = p
	return r, nil
}
func (m *Manager) state(r *room, client string, after int64) State {
	s := State{Room: r.id, Client: client, Revision: r.revision, Saved: r.saved, Sequence: r.sequence, Events: []Event{}, Participants: []Participant{}}
	for _, e := range r.events {
		if e.Seq > after {
			s.Events = append(s.Events, e)
		}
	}
	for _, p := range r.clients {
		s.Participants = append(s.Participants, p)
	}
	return s
}
func (m *Manager) Sync(path, roomID, client string, after int64) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return State{}, err
	}
	return m.state(r, client, after), nil
}
func (m *Manager) Base(path, roomID, client string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), r.base...), nil
}
func (m *Manager) Acquire(path, roomID, client string, revision int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return "", err
	}
	if !r.clients[client].Edit {
		return "", os.ErrPermission
	}
	if revision != r.revision || (r.lease != "" && r.leaseOwner != client) {
		return "", ErrConflict
	}
	if r.lease == "" {
		r.lease = uuid.NewString()
		r.leaseOwner = client
	}
	r.leaseUntil = time.Now().Add(15 * time.Second)
	return r.lease, nil
}
func (m *Manager) Change(path, roomID, client, batch, lease string, changes []json.RawMessage, extra json.RawMessage) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return State{}, err
	}
	if !r.clients[client].Edit {
		return State{}, os.ErrPermission
	}
	if _, ok := r.batches[client+":"+batch]; ok {
		return m.state(r, client, 0), nil
	}
	if batch == "" || r.lease != lease || r.leaseOwner != client || lease == "" {
		return State{}, ErrConflict
	}
	size := 0
	for _, c := range changes {
		size += len(c)
		var str string
		if json.Unmarshal(c, &str) != nil {
			return State{}, fmt.Errorf("invalid native change")
		}
	}
	if size > 4<<20 || r.bytes+size > 32<<20 {
		return State{}, fmt.Errorf("session change limit reached; save and reopen before continuing")
	}
	r.revision++
	r.sequence++
	r.bytes += size
	r.events = append(r.events, Event{Seq: r.sequence, Revision: r.revision, Type: "changes", Client: client, Author: r.clients[client].Name, Changes: changes, Extra: extra})
	r.batches[client+":"+batch] = r.revision
	if err := m.persist(r); err != nil {
		delete(r.batches, client+":"+batch)
		r.events = r.events[:len(r.events)-1]
		r.revision--
		r.sequence--
		r.bytes -= size
		return State{}, err
	}
	m.releaseLocks(r, client)
	r.lease = ""
	r.leaseOwner = ""
	return m.state(r, client, 0), nil
}
func (m *Manager) Cursor(path, roomID, client, cursor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if len(cursor) > 65536 {
		return fmt.Errorf("cursor is too large")
	}
	// Keep only the most recent cursor per participant. Changes remain ordered.
	filtered := r.events[:0]
	for _, e := range r.events {
		if !(e.Type == "cursor" && e.Client == client) {
			filtered = append(filtered, e)
		}
	}
	r.events = filtered
	r.sequence++
	r.events = append(r.events, Event{Seq: r.sequence, Revision: r.revision, Type: "cursor", Client: client, Cursor: cursor})
	return nil
}
func (m *Manager) Release(path, roomID, client, lease string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if r.leaseOwner == client && (lease == "" || r.lease == lease) {
		r.lease = ""
		r.leaseOwner = ""
	}
	m.releaseLocks(r, client)
	return nil
}

// Checkpoint holds the session mutex throughout the filesystem write, so a
// snapshot cannot race a change batch. The supplied writer retains FileBrowser's
// normal quota, permissions and write behavior.
func (m *Manager) Checkpoint(path, roomID, client, lease string, revision int64, write func() error) error {
	return m.CheckpointWithHistory(path, roomID, client, lease, revision, func(_ []byte, _ []string) error { return write() })
}
func (m *Manager) CheckpointWithHistory(path, roomID, client, lease string, revision int64, write func([]byte, []string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if !r.clients[client].Edit {
		return os.ErrPermission
	}
	if r.leaseOwner != client || lease == "" || r.lease != lease || revision != r.revision {
		return ErrConflict
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if fingerprint(current) != r.digest {
		return fmt.Errorf("file changed outside this session; download your draft before reopening")
	}
	if revision == r.saved {
		r.lease = ""
		r.leaseOwner = ""
		return nil
	}
	authors := []string{}
	seen := map[string]bool{}
	for _, e := range r.events {
		if e.Type == "changes" && e.Revision > r.saved && e.Author != "" && !seen[e.Author] {
			authors = append(authors, e.Author)
			seen[e.Author] = true
		}
	}
	if err = write(current, authors); err != nil {
		return err
	}
	current, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	r.digest = fingerprint(current)
	r.saved = revision
	if err = m.persist(r); err != nil {
		return err
	}
	r.lease = ""
	r.leaseOwner = ""
	return nil
}

// Restore takes the same save lease as checkpointing, and retires the old
// session so its clients cannot overwrite the restored file from stale models.
func (m *Manager) Restore(path, roomID, client, lease string, revision int64, write func([]byte) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	if !r.clients[client].Edit {
		return os.ErrPermission
	}
	if r.leaseOwner != client || lease == "" || r.lease != lease || revision != r.revision || r.saved != r.revision {
		return ErrConflict
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if fingerprint(current) != r.digest {
		return fmt.Errorf("file changed outside this session; download your draft before reopening")
	}
	if err = write(current); err != nil {
		return err
	}
	m.removeJournal(path)
	delete(m.rooms, path)
	return nil
}
func (m *Manager) ActiveUnder(path string, isDir bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rooms {
		m.expire(r)
		if len(r.clients) == 0 {
			continue
		}
		relative, err := filepath.Rel(path, r.path)
		if err == nil && (relative == "." || (isDir && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))) {
			return true
		}
	}
	return false
}
func (m *Manager) Leave(path, roomID, client string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.get(path, roomID, client)
	if err != nil {
		return err
	}
	delete(r.clients, client)
	m.releaseLocks(r, client)
	if r.leaseOwner == client {
		r.lease = ""
		r.leaseOwner = ""
	}
	return nil
}
