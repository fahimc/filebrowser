package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"os"
	"path/filepath"
	"testing"
)

func TestOfficeRecoveryCacheIsNeverExposed(t *testing.T) {
	previous := settings.Config.Server.CacheDir
	defer func() { settings.Config.Server.CacheDir = previous }()
	source := t.TempDir()
	settings.Config.Server.CacheDir = filepath.Join(source, "cache")
	dir := filepath.Join(settings.Config.Server.CacheDir, "office-sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "private.json"), []byte("private recovery data"), 0600)
	for _, name := range []string{"alice", "admin", ""} {
		if AccessPermitted(source, utils.IndexPathFromNormalized("/cache/office-sessions/private.json", false), name) {
			t.Fatalf("recovery data exposed to %q", name)
		}
	}
}
func TestOfficeHistoryIsNeverExposed(t *testing.T) {
	previous := settings.Config.Server.OfficeHistoryDir
	defer func() { settings.Config.Server.OfficeHistoryDir = previous }()
	source := t.TempDir()
	settings.Config.Server.OfficeHistoryDir = filepath.Join(source, "persistent-history")
	dir := filepath.Join(settings.Config.Server.OfficeHistoryDir, "documents")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "private.docx"), []byte("historical private content"), 0600)
	for _, name := range []string{"alice", "admin", ""} {
		if AccessPermitted(source, utils.IndexPathFromNormalized("/persistent-history/documents/private.docx", false), name) {
			t.Fatalf("history data exposed to %q", name)
		}
	}
	link := filepath.Join(source, "linked-history")
	if err := os.Symlink(dir, link); err == nil {
		if AccessPermitted(source, utils.IndexPathFromNormalized("/linked-history/private.docx", false), "admin") {
			t.Fatal("history exposed through symlink")
		}
	}
}
