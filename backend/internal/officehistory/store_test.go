package officehistory

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func historyFixture(t *testing.T) (*Store, string, *time.Time) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "report.docx")
	if err := os.WriteFile(path, []byte("Original document"), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(root, "history"), 0)
	clock := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	return s, path, &clock
}
func save(t *testing.T, s *Store, path, text string, people []string, rev int64) {
	t.Helper()
	before, _ := os.ReadFile(path)
	if err := s.Commit(path, before, []byte(text), people, "room", rev, "", "", func(body []byte) error { return os.WriteFile(path, body, 0600) }); err != nil {
		t.Fatal(err)
	}
}
func TestSavepointsNamesAndPersistence(t *testing.T) {
	s, path, clock := historyFixture(t)
	initial, err := s.List(path)
	if err != nil || len(initial) != 1 {
		t.Fatal(initial, err)
	}
	save(t, s, path, "First edit", []string{"Alice"}, 1)
	versions, _ := s.List(path)
	first := versions[0]
	*clock = clock.Add(time.Minute)
	save(t, s, path, "Merged edit", []string{"Bob"}, 2)
	versions, _ = s.List(path)
	if len(versions) != 2 || versions[0].ID != first.ID || len(versions[0].Authors) != 2 {
		t.Fatal("autosave grouping lost attribution", versions)
	}
	if _, _, err = s.Read(path, first.ID, first.Digest); !errors.Is(err, ErrChanged) {
		t.Fatal("stale preview was not rejected", err)
	}
	if _, err = s.Name(path, versions[0].ID, versions[0].Digest, "Approved draft"); err != nil {
		t.Fatal(err)
	}
	save(t, s, path, "Next edit", []string{"Alice"}, 3)
	restarted := New(s.dir, 0)
	versions, err = restarted.List(path)
	if err != nil || len(versions) != 3 || versions[1].Name != "Approved draft" {
		t.Fatal(versions, err)
	}
	_, body, err := restarted.Read(path, initial[0].ID, initial[0].Digest)
	if err != nil || string(body) != "Original document" {
		t.Fatal("original snapshot lost", err)
	}
}
func TestRestoreAndFailedWritePreserveVersions(t *testing.T) {
	s, path, _ := historyFixture(t)
	original, _ := s.List(path)
	save(t, s, path, "Current document", []string{"Alice"}, 1)
	current, _ := os.ReadFile(path)
	fail := errors.New("disk failure")
	if err := s.Commit(path, current, nil, []string{"Bob"}, "room", 1, original[0].ID, original[0].Digest, func([]byte) error { return fail }); !errors.Is(err, fail) {
		t.Fatal(err)
	}
	v, _ := s.List(path)
	if len(v) != 2 || s.Restored(path, "room") {
		t.Fatal("failed restore became visible")
	}
	if err := s.Commit(path, current, nil, []string{"Bob"}, "room", 1, original[0].ID, original[0].Digest, func(body []byte) error { return os.WriteFile(path, body, 0600) }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.List(path)
	if len(v) != 3 || v[0].Kind != "restore" || v[0].RestoredFrom != original[0].ID || !s.Restored(path, "room") {
		t.Fatal(v)
	}
	_, body, err := s.Read(path, v[1].ID, v[1].Digest)
	if err != nil || string(body) != "Current document" {
		t.Fatal("pre-restore document lost", err)
	}
	if !New(s.dir, 0).Restored(path, "room") {
		t.Fatal("restore signal did not survive restart")
	}
}
func TestPendingCrashRecovery(t *testing.T) {
	for _, afterWrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "before write", true: "after write"}[afterWrite], func(t *testing.T) {
			s, path, _ := historyFixture(t)
			save(t, s, path, "Last saved", []string{"Alice"}, 1)
			old, _ := s.List(path)
			before, _ := os.ReadFile(path)
			func() {
				defer func() { recover() }()
				s.Commit(path, before, []byte("New draft"), []string{"Bob"}, "room", 2, "", "", func(body []byte) error {
					if afterWrite {
						os.WriteFile(path, body, 0600)
					}
					panic("process crash")
				})
			}()
			fresh := New(s.dir, 0)
			versions, err := fresh.List(path)
			if err != nil {
				t.Fatal(err)
			}
			_, body, err := fresh.Read(path, versions[0].ID, versions[0].Digest)
			if err != nil {
				t.Fatal(err)
			}
			expected := "Last saved"
			if afterWrite {
				expected = "New draft"
			}
			if string(body) != expected || versions[0].ID != old[0].ID {
				t.Fatal("pending merge recovery corrupted a version", versions, string(body))
			}
		})
	}
}
func TestRetentionKeepsNamedAndOriginal(t *testing.T) {
	s, path, clock := historyFixture(t)
	s.limit = 2
	save(t, s, path, "Named", []string{"Alice"}, 1)
	v, _ := s.List(path)
	namedID := v[0].ID
	s.Name(path, namedID, v[0].Digest, "Keep forever")
	for i := 0; i < 5; i++ {
		*clock = clock.Add(6 * time.Minute)
		save(t, s, path, string(rune('a'+i)), []string{"Bob"}, int64(i+2))
	}
	v, _ = s.List(path)
	if len(v) != 4 {
		t.Fatal("retention limit did not preserve original and named versions", v)
	}
	if _, _, err := s.Read(path, namedID, ""); err != nil {
		t.Fatal(err)
	}
}
func TestRenameAndDirectoryMoveKeepHistoryIdentity(t *testing.T) {
	s, path, _ := historyFixture(t)
	save(t, s, path, "Edited", []string{"Alice"}, 1)
	v, _ := s.List(path)
	target := filepath.Join(filepath.Dir(path), "renamed.docx")
	if err := s.Move(path, target, false, func() error { return os.Rename(path, target) }); err != nil {
		t.Fatal(err)
	}
	renamed, err := s.List(target)
	if err != nil || renamed[0].ID != v[0].ID {
		t.Fatal(renamed, err)
	}
	folder := filepath.Join(filepath.Dir(path), "folder")
	os.MkdirAll(folder, 0700)
	next := filepath.Join(folder, "renamed.docx")
	if err = s.Move(target, next, false, func() error { return os.Rename(target, next) }); err != nil {
		t.Fatal(err)
	}
	newFolder := filepath.Join(filepath.Dir(path), "moved")
	if err = s.Move(folder, newFolder, true, func() error { return os.Rename(folder, newFolder) }); err != nil {
		t.Fatal(err)
	}
	result, err := s.List(filepath.Join(newFolder, "renamed.docx"))
	if err != nil || result[0].ID != v[0].ID {
		t.Fatal(result, err)
	}
}
func TestSnapshotIntegrityAndIsolation(t *testing.T) {
	s, path, _ := historyFixture(t)
	versions, _ := s.List(path)
	if _, _, err := s.Read(path, "../../private", ""); err == nil {
		t.Fatal("invalid snapshot identifier accepted")
	}
	other := filepath.Join(filepath.Dir(path), "other.docx")
	os.WriteFile(other, []byte("Other file"), 0600)
	s.List(other)
	if _, _, err := s.Read(other, versions[0].ID, ""); err == nil {
		t.Fatal("another file's snapshot was exposed")
	}
	d, _ := s.load(path, false)
	os.WriteFile(s.blob(d, versions[0]), []byte("tampered"), 0600)
	if _, _, err := s.Read(path, versions[0].ID, ""); err == nil {
		t.Fatal("tampered snapshot was accepted")
	}
}

func TestResetMarkersSurviveAutomaticRetention(t *testing.T) {
	s, path, clock := historyFixture(t)
	s.limit = 1
	versions, _ := s.List(path)
	save(t, s, path, "Edited", []string{"Alice"}, 1)
	before, _ := os.ReadFile(path)
	err := s.Commit(path, before, nil, []string{"Alice"}, "retired-room", 1, versions[0].ID, versions[0].Digest, func(data []byte) error { return os.WriteFile(path, data, 0600) })
	if err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(6 * time.Minute)
	save(t, s, path, "Later", []string{"Bob"}, 2)
	fresh := New(s.dir, 1)
	if !fresh.Restored(path, "retired-room") {
		t.Fatal("retired room can write after its restore version is pruned")
	}
}

func TestCreatedFileDoesNotReuseDeletedFileHistory(t *testing.T) {
	s, path, _ := historyFixture(t)
	old, _ := s.List(path)
	os.Remove(path)
	os.WriteFile(path, []byte("Different document"), 0600)
	if err := s.Created(path, "Bob"); err != nil {
		t.Fatal(err)
	}
	current, err := s.List(path)
	if err != nil || len(current) != 1 || current[0].ID == old[0].ID {
		t.Fatal(current, err)
	}
	if _, _, err = s.Read(path, old[0].ID, old[0].Digest); err == nil {
		t.Fatal("old document history exposed at reused filename")
	}
}
