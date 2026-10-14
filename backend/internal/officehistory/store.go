// Package officehistory stores document savepoints as ordinary Office files.
// JSON indexes contain metadata only; document bodies never enter a database.
package officehistory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrChanged = errors.New("this version changed; refresh version history before continuing")
var ErrDeferred = errors.New("the document was saved; history metadata will be recovered on the next request")

const MaxDocumentBytes = 100 << 20

type Version struct {
	ID           string    `json:"id"`
	Created      time.Time `json:"created"`
	Started      time.Time `json:"started"`
	Name         string    `json:"name"`
	Authors      []string  `json:"authors"`
	Kind         string    `json:"kind"`
	Digest       string    `json:"digest"`
	Format       string    `json:"format"`
	Size         int       `json:"size"`
	Room         string    `json:"room,omitempty"`
	Revision     int64     `json:"revision,omitempty"`
	RestoredFrom string    `json:"restoredFrom,omitempty"`
	ResetRoom    string    `json:"resetRoom,omitempty"`
	Pending      bool      `json:"pending,omitempty"`
}
type document struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Versions   []Version `json:"versions"`
	Rollback   []Version `json:"rollback,omitempty"`
	ResetRooms []string  `json:"resetRooms,omitempty"`
}
type pointer struct {
	ID string `json:"id"`
}
type Store struct {
	mu     sync.Mutex
	dir    string
	limit  int
	now    func() time.Time
	resets map[string]map[string]bool
}

func New(dir string, limit int) *Store {
	return &Store{dir: dir, limit: limit, now: time.Now, resets: map[string]map[string]bool{}}
}
func (s *Store) Configure(dir string, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir != dir {
		s.resets = map[string]map[string]bool{}
	}
	s.dir = dir
	s.limit = limit
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func normalized(path string) string {
	path, _ = filepath.Abs(path)
	if actual, err := filepath.EvalSymlinks(path); err == nil {
		path = actual
	}
	return filepath.Clean(path)
}
func key(path string) string {
	path = normalized(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return digest([]byte(path))
}
func supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx", ".xlsx", ".pptx", ".pdf":
		return true
	}
	return false
}
func readDocument(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() || info.Size() > MaxDocumentBytes {
		return nil, fmt.Errorf("version history supports Office files up to 100 MiB")
	}
	return os.ReadFile(path)
}
func (s *Store) pointerPath(path string) string {
	return filepath.Join(s.dir, "paths", key(path)+".json")
}
func (s *Store) folder(id string) string    { return filepath.Join(s.dir, "documents", id) }
func (s *Store) manifest(d document) string { return filepath.Join(s.folder(d.ID), "index.json") }
func atomicJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".index-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
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
	return os.Rename(tmp, path)
}
func readJSON(path string, value any) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > 32<<20 {
		return fmt.Errorf("history index is too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
func (s *Store) load(path string, create bool) (document, error) {
	var p pointer
	err := readJSON(s.pointerPath(path), &p)
	if os.IsNotExist(err) && create {
		p.ID = uuid.NewString()
		d := document{ID: p.ID, Path: normalized(path), Versions: []Version{}}
		if err = atomicJSON(s.manifest(d), d); err != nil {
			return d, err
		}
		err = atomicJSON(s.pointerPath(path), p)
		return d, err
	}
	if err != nil {
		return document{}, err
	}
	if _, err = uuid.Parse(p.ID); err != nil {
		return document{}, fmt.Errorf("invalid document history identity")
	}
	var d document
	if err = readJSON(filepath.Join(s.folder(p.ID), "index.json"), &d); err != nil {
		return d, err
	}
	if d.ID != p.ID {
		return d, fmt.Errorf("history identity mismatch")
	}
	// Pending entries recover after a crash by checking the ordinary current file.
	changed := false
	for _, v := range d.Versions {
		changed = changed || v.Pending
	}
	if !changed {
		return d, nil
	}
	current, err := readDocument(path)
	if err != nil {
		return d, err
	}
	sum := digest(current)
	for _, v := range d.Versions {
		if v.Pending && v.Digest != sum && len(d.Rollback) > 0 {
			d.Versions = d.Rollback
			d.Rollback = nil
			return d, atomicJSON(s.manifest(d), d)
		}
	}
	versions := d.Versions[:0]
	for _, v := range d.Versions {
		if v.Pending {
			changed = true
			if v.Digest != sum {
				continue
			}
			v.Pending = false
			if v.ResetRoom != "" {
				d.ResetRooms = append(d.ResetRooms, v.ResetRoom)
			}
		}
		versions = append(versions, v)
	}
	d.Versions = versions
	d.Rollback = nil
	if changed {
		if err = atomicJSON(s.manifest(d), d); err != nil {
			return d, err
		}
	}
	return d, nil
}
func (s *Store) blob(d document, v Version) string {
	return filepath.Join(s.folder(d.ID), v.Digest+v.Format)
}
func (s *Store) putBlob(d document, data []byte, format string) (string, error) {
	if !supported("file"+format) || len(data) > MaxDocumentBytes {
		return "", fmt.Errorf("unsupported history snapshot")
	}
	sum := digest(data)
	path := filepath.Join(s.folder(d.ID), sum+format)
	if existing, err := os.ReadFile(path); err == nil {
		if digest(existing) != sum {
			return "", fmt.Errorf("history snapshot integrity check failed")
		}
		return sum, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return "", err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return sum, os.Rename(tmp, path)
}
func authors(values ...[]string) []string {
	seen := map[string]bool{}
	for _, group := range values {
		for _, name := range group {
			if name != "" {
				seen[name] = true
			}
		}
	}
	out := []string{}
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
func (s *Store) ensure(d *document, path string, current []byte) error {
	sum := digest(current)
	if len(d.Versions) > 0 && d.Versions[len(d.Versions)-1].Digest == sum {
		return nil
	}
	format := strings.ToLower(filepath.Ext(path))
	if _, err := s.putBlob(*d, current, format); err != nil {
		return err
	}
	kind := "external"
	if len(d.Versions) == 0 {
		kind = "initial"
	}
	now := s.now().UTC()
	d.Versions = append(d.Versions, Version{ID: uuid.NewString(), Created: now, Started: now, Kind: kind, Digest: sum, Format: format, Size: len(current), Authors: []string{}})
	return atomicJSON(s.manifest(*d), *d)
}
func (s *Store) List(path string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(path, true)
	if err != nil {
		return nil, err
	}
	data, err := readDocument(path)
	if err != nil {
		return nil, err
	}
	if err = s.ensure(&d, path, data); err != nil {
		return nil, err
	}
	s.prune(&d)
	result := append([]Version{}, d.Versions...)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// Created starts a fresh identity when a new file is created at a previously
// deleted path. Reusing a filename must not expose another file's old history.
func (s *Store) Created(path, author string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := readDocument(path)
	if err != nil {
		return err
	}
	d := document{ID: uuid.NewString(), Path: normalized(path), Versions: []Version{}}
	if err = s.ensure(&d, path, data); err != nil {
		return err
	}
	d.Versions[0].Authors = authors([]string{author})
	if err = atomicJSON(s.manifest(d), d); err != nil {
		return err
	}
	s.resets[key(path)] = nil
	return atomicJSON(s.pointerPath(path), pointer{d.ID})
}
func find(d document, id, expected string) (Version, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Version{}, os.ErrNotExist
	}
	for _, v := range d.Versions {
		if v.ID == id && !v.Pending {
			if expected != "" && v.Digest != expected {
				return Version{}, ErrChanged
			}
			return v, nil
		}
	}
	return Version{}, os.ErrNotExist
}
func (s *Store) readBlob(d document, v Version) ([]byte, error) {
	if len(v.Digest) != 64 || !supported("file"+v.Format) {
		return nil, fmt.Errorf("invalid snapshot metadata")
	}
	if _, err := hex.DecodeString(v.Digest); err != nil {
		return nil, err
	}
	body, err := readDocument(s.blob(d, v))
	if err != nil {
		return nil, err
	}
	if digest(body) != v.Digest {
		return nil, fmt.Errorf("history snapshot integrity check failed")
	}
	return body, nil
}
func (s *Store) Read(path, id, expected string) (Version, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(path, false)
	if err != nil {
		return Version{}, nil, err
	}
	v, err := find(d, id, expected)
	if err != nil {
		return v, nil, err
	}
	body, err := s.readBlob(d, v)
	return v, body, err
}
func (s *Store) Name(path, id, expected, name string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 120 {
		return Version{}, fmt.Errorf("version names must be 120 characters or fewer")
	}
	d, err := s.load(path, false)
	if err != nil {
		return Version{}, err
	}
	v, err := find(d, id, expected)
	if err != nil {
		return v, err
	}
	count := 0
	for _, entry := range d.Versions {
		if entry.Name != "" && entry.ID != id {
			count++
		}
	}
	if name != "" && count >= 40 {
		return v, fmt.Errorf("keep up to 40 named versions; remove another name first")
	}
	for i := range d.Versions {
		if d.Versions[i].ID == id {
			d.Versions[i].Name = name
			v = d.Versions[i]
			break
		}
	}
	err = atomicJSON(s.manifest(d), d)
	return v, err
}

// Commit durably prepares both snapshots before the normal quota-aware write.
// The pending flag makes a crash before/after the filesystem write recoverable.
func (s *Store) Commit(path string, before, next []byte, people []string, room string, revision int64, restoredFrom, expected string, write func([]byte) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(path, true)
	if err != nil {
		return err
	}
	if err = s.ensure(&d, path, before); err != nil {
		return err
	}
	old := d
	old.Versions = append([]Version{}, d.Versions...)
	kind := "edit"
	if restoredFrom != "" {
		selected, findErr := find(d, restoredFrom, expected)
		if findErr != nil {
			return findErr
		}
		if selected.Format != strings.ToLower(filepath.Ext(path)) {
			return fmt.Errorf("this version uses a different format; download it instead")
		}
		next, err = s.readBlob(d, selected)
		if err != nil {
			return err
		}
		kind = "restore"
	}
	format := strings.ToLower(filepath.Ext(path))
	sum, err := s.putBlob(d, next, format)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	v := Version{ID: uuid.NewString(), Created: now, Started: now, Kind: kind, Digest: sum, Format: format, Size: len(next), Authors: authors(people), Room: room, Revision: revision, RestoredFrom: restoredFrom, Pending: true}
	if kind == "restore" {
		v.ResetRoom = room
	}
	if kind == "edit" && len(d.Versions) > 0 {
		last := d.Versions[len(d.Versions)-1]
		if last.Kind == "edit" && last.Name == "" && last.Room == room && now.Sub(last.Started) < 5*time.Minute {
			v.ID = last.ID
			v.Started = last.Started
			v.Authors = authors(last.Authors, v.Authors)
			d.Versions = d.Versions[:len(d.Versions)-1]
		}
	}
	d.Rollback = old.Versions
	d.Versions = append(d.Versions, v)
	if err = atomicJSON(s.manifest(d), d); err != nil {
		return err
	}
	if err = write(next); err != nil {
		rollback := atomicJSON(s.manifest(old), old)
		return errors.Join(err, rollback)
	}
	d.Versions[len(d.Versions)-1].Pending = false
	d.Rollback = nil
	if kind == "restore" {
		d.ResetRooms = append(d.ResetRooms, room)
		s.resets[key(path)] = nil
	}
	if err = atomicJSON(s.manifest(d), d); err != nil {
		return ErrDeferred
	}
	s.prune(&d)
	return nil
}
func (s *Store) prune(d *document) {
	if s.limit > 0 {
		automatic := 0
		for _, v := range d.Versions {
			if v.Name == "" && v.Kind != "initial" {
				automatic++
			}
		}
		keep := d.Versions[:0]
		for _, v := range d.Versions {
			if automatic > s.limit && v.Name == "" && v.Kind != "initial" {
				automatic--
				continue
			}
			keep = append(keep, v)
		}
		d.Versions = keep
		if atomicJSON(s.manifest(*d), *d) != nil {
			return
		}
	}
	// Old blobs from merged autosaves are removed only after committed metadata.
	used := map[string]bool{}
	for _, v := range d.Versions {
		used[filepath.Base(s.blob(*d, v))] = true
	}
	entries, _ := os.ReadDir(s.folder(d.ID))
	for _, entry := range entries {
		if !entry.IsDir() && supported(entry.Name()) && !used[entry.Name()] {
			os.Remove(filepath.Join(s.folder(d.ID), entry.Name()))
		}
	}
}
func (s *Store) Restored(path, room string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(path)
	cached, ok := s.resets[k]
	if ok && cached != nil {
		return cached[room]
	}
	d, err := s.load(path, false)
	if err != nil {
		return false
	}
	cached = map[string]bool{}
	for _, roomID := range d.ResetRooms {
		cached[roomID] = true
	}
	for _, v := range d.Versions {
		if v.Kind == "restore" {
			cached[v.ResetRoom] = true
		}
	}
	s.resets[k] = cached
	return cached[room]
}

// Move preserves identity and history when FileBrowser renames/moves a file or
// directory. The store mutex prevents readers observing the staged path map.
func (s *Store) Move(from, to string, isDir bool, write func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	from = normalized(from)
	to = normalized(to)
	type change struct {
		oldPath, newPath string
		doc              document
		previous         []byte
	}
	changes := []change{}
	entries, err := os.ReadDir(filepath.Join(s.dir, "documents"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := uuid.Parse(entry.Name()); err != nil {
			continue
		}
		var d document
		if readJSON(filepath.Join(s.folder(entry.Name()), "index.json"), &d) != nil {
			continue
		}
		var p pointer
		if readJSON(s.pointerPath(d.Path), &p) != nil || p.ID != d.ID {
			continue
		}
		relative, err := filepath.Rel(from, d.Path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || (!isDir && relative != ".") {
			continue
		}
		dest := to
		if isDir {
			dest = filepath.Join(to, relative)
		}
		previous, _ := os.ReadFile(s.pointerPath(dest))
		changes = append(changes, change{d.Path, dest, d, previous})
	}
	rollback := func() {
		for _, c := range changes {
			if len(c.previous) == 0 {
				os.Remove(s.pointerPath(c.newPath))
			} else {
				var p pointer
				if json.Unmarshal(c.previous, &p) == nil {
					atomicJSON(s.pointerPath(c.newPath), p)
				}
			}
		}
	}
	for _, c := range changes {
		if err = atomicJSON(s.pointerPath(c.newPath), pointer{c.doc.ID}); err != nil {
			rollback()
			return err
		}
	}
	if err = write(); err != nil {
		rollback()
		return err
	}
	for _, c := range changes {
		if key(c.oldPath) != key(c.newPath) {
			os.Remove(s.pointerPath(c.oldPath))
		}
		c.doc.Path = c.newPath
		if err = atomicJSON(s.manifest(c.doc), c.doc); err != nil {
			return err
		}
	}
	return nil
}
