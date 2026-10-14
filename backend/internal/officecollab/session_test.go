package officecollab

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (*Manager, string, State, State) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "document.docx")
	if err := os.WriteFile(path, []byte("ordinary file"), 0600); err != nil {
		t.Fatal(err)
	}
	m := New()
	a, err := m.Join(path, "Alice", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Join(path, "Bob", true)
	if err != nil {
		t.Fatal(err)
	}
	return m, path, a, b
}
func TestOrderedChangesAndIdempotentRetry(t *testing.T) {
	m, path, a, b := fixture(t)
	lease, err := m.Acquire(path, a.Room, a.Client, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Acquire(path, b.Room, b.Client, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent lease granted: %v", err)
	}
	changes := []json.RawMessage{json.RawMessage(`"native-change"`)}
	first, err := m.Change(path, a.Room, a.Client, "batch", lease, changes, nil)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := m.Change(path, a.Room, a.Client, "batch", lease, changes, nil)
	if err != nil || retry.Revision != first.Revision {
		t.Fatalf("duplicated retry: %v %+v", err, retry)
	}
	state, err := m.Sync(path, b.Room, b.Client, 0)
	if err != nil || len(state.Events) != 1 || state.Events[0].Client != a.Client {
		t.Fatalf("lost ordered change: %v %+v", err, state)
	}
	if _, err = m.Acquire(path, b.Room, b.Client, 0); !errors.Is(err, ErrConflict) {
		t.Fatal("stale revision obtained lease")
	}
}
func TestCheckpointRevisionAndExternalFileProtection(t *testing.T) {
	m, path, a, b := fixture(t)
	lease, _ := m.Acquire(path, a.Room, a.Client, 0)
	_, err := m.Change(path, a.Room, a.Client, "1", lease, []json.RawMessage{json.RawMessage(`"edit"`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lease, _ = m.Acquire(path, a.Room, a.Client, 1)
	writes := 0
	write := func() error { writes++; return os.WriteFile(path, []byte("merged ordinary file"), 0600) }
	if err = m.Checkpoint(path, a.Room, a.Client, lease, 0, write); !errors.Is(err, ErrConflict) || writes != 0 {
		t.Fatal("stale checkpoint wrote a file")
	}
	if err = m.Checkpoint(path, a.Room, a.Client, lease, 1, write); err != nil {
		t.Fatal(err)
	}
	state, _ := m.Sync(path, b.Room, b.Client, 0)
	if state.Saved != 1 {
		t.Fatal("checkpoint not visible to other devices")
	}
	baseline, _ := m.Base(path, b.Room, b.Client)
	if string(baseline) != "ordinary file" {
		t.Fatal("late join baseline changed")
	}
	lease, _ = m.Acquire(path, a.Room, a.Client, 1)
	os.WriteFile(path, []byte("outside edit"), 0600)
	if err = m.Checkpoint(path, a.Room, a.Client, lease, 1, write); err == nil || writes != 1 {
		t.Fatal("external edit overwritten")
	}
}
func TestReadonlyAndSessionIsolation(t *testing.T) {
	m, path, a, _ := fixture(t)
	viewer, _ := m.Join(path, "Viewer", false)
	if _, err := m.Acquire(path, viewer.Room, viewer.Client, 0); !errors.Is(err, os.ErrPermission) {
		t.Fatal("view-only session can write")
	}
	if _, err := m.Sync(path, "other-room", a.Client, 0); !errors.Is(err, ErrSession) {
		t.Fatal("room isolation failed")
	}
	if _, err := m.Sync(path, a.Room, "other-device", 0); !errors.Is(err, ErrSession) {
		t.Fatal("device isolation failed")
	}
}

func TestRecoveryJournalAndResume(t *testing.T) {
	m, path, a, _ := fixture(t)
	dir := t.TempDir()
	m.SetJournalDir(dir)
	if err := m.BindOwner(path, a.Room, a.Client, "user:alice"); err != nil {
		t.Fatal(err)
	}
	lease, _ := m.Acquire(path, a.Room, a.Client, 0)
	if _, err := m.Change(path, a.Room, a.Client, "durable", lease, []json.RawMessage{json.RawMessage(`"change"`)}, nil); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.SetJournalDir(dir)
	recovered, err := restarted.Resume(path, a.Room, a.Client, "user:alice", "Alice", true, 0)
	if err != nil || recovered.Revision != 1 || len(recovered.Events) != 1 {
		t.Fatalf("recovery failed: %v %+v", err, recovered)
	}
	if _, err = restarted.Resume(path, a.Room, a.Client, "user:mallory", "Mallory", true, 0); !errors.Is(err, os.ErrPermission) {
		t.Fatal("another user resumed the private session")
	}
	lease, _ = restarted.Acquire(path, a.Room, a.Client, 1)
	retry, err := restarted.Change(path, a.Room, a.Client, "durable", lease, []json.RawMessage{json.RawMessage(`"change"`)}, nil)
	if err != nil || retry.Revision != 1 {
		t.Fatal("recovery duplicated an acknowledged batch")
	}
}
func TestEditingRangeLocks(t *testing.T) {
	m, path, a, b := fixture(t)
	left := json.RawMessage(`{"sheetId":"sheet","type":1,"guid":"a","rangeOrObjectId":{"r1":0,"r2":2,"c1":0,"c2":2}}`)
	overlap := json.RawMessage(`{"sheetId":"sheet","type":1,"guid":"b","rangeOrObjectId":{"r1":1,"r2":3,"c1":1,"c2":3}}`)
	separate := json.RawMessage(`{"sheetId":"sheet","type":1,"guid":"c","rangeOrObjectId":{"r1":4,"r2":4,"c1":4,"c2":4}}`)
	if err := m.Locks(path, a.Room, a.Client, []json.RawMessage{left}); err != nil {
		t.Fatal(err)
	}
	if err := m.Locks(path, b.Room, b.Client, []json.RawMessage{overlap}); !errors.Is(err, ErrConflict) {
		t.Fatal("overlapping range lock granted")
	}
	if err := m.Locks(path, b.Room, b.Client, []json.RawMessage{separate}); err != nil {
		t.Fatal(err)
	}
	m.Release(path, a.Room, a.Client, "")
	if err := m.Locks(path, b.Room, b.Client, []json.RawMessage{overlap}); err != nil {
		t.Fatal("editing lock was not released")
	}
}
func TestHistoryRestoreRetiresSessionAndRequiresCheckpoint(t *testing.T) {
	m, path, a, b := fixture(t)
	lease, _ := m.Acquire(path, a.Room, a.Client, 0)
	if _, err := m.Change(path, a.Room, a.Client, "change", lease, []json.RawMessage{json.RawMessage(`"edit"`)}, nil); err != nil {
		t.Fatal(err)
	}
	lease, _ = m.Acquire(path, a.Room, a.Client, 1)
	if err := m.Restore(path, a.Room, a.Client, lease, 1, func([]byte) error { t.Fatal("unsaved revision restored"); return nil }); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := m.Checkpoint(path, a.Room, a.Client, lease, 1, func() error { return os.WriteFile(path, []byte("current saved"), 0600) }); err != nil {
		t.Fatal(err)
	}
	lease, _ = m.Acquire(path, a.Room, a.Client, 1)
	if err := m.Restore(path, a.Room, a.Client, lease, 1, func(current []byte) error {
		if string(current) != "current saved" {
			t.Fatal("missing pre-restore version")
		}
		return os.WriteFile(path, []byte("restored ordinary file"), 0600)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Sync(path, b.Room, b.Client, 0); !errors.Is(err, ErrSession) {
		t.Fatal("stale device session remained writable", err)
	}
	next, err := m.Join(path, "Alice", true)
	if err != nil || next.Room == a.Room {
		t.Fatal(next, err)
	}
	baseline, _ := m.Base(path, next.Room, next.Client)
	if string(baseline) != "restored ordinary file" {
		t.Fatal("new session did not load restored bytes")
	}
}
