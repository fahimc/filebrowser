package settings

import (
	"path/filepath"
	"testing"
)

func TestOfficeHistoryDirectoryDefaultsBesideDatabase(t *testing.T) {
	previousDir, previousDB := Config.Server.OfficeHistoryDir, Config.Server.DatabaseV2.Path
	defer func() { Config.Server.OfficeHistoryDir = previousDir; Config.Server.DatabaseV2.Path = previousDB }()
	Config.Server.OfficeHistoryDir = ""
	Config.Server.DatabaseV2.Path = filepath.Join(t.TempDir(), "settings.sqlite")
	expected := filepath.Join(filepath.Dir(Config.Server.DatabaseV2.Path), ".filebrowser-history")
	if OfficeHistoryDirectory() != expected {
		t.Fatal(OfficeHistoryDirectory(), expected)
	}
	override := filepath.Join(t.TempDir(), "persistent")
	Config.Server.OfficeHistoryDir = override
	if OfficeHistoryDirectory() != override {
		t.Fatal("history override ignored")
	}
}
