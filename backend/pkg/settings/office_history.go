package settings

import "path/filepath"

// OfficeHistoryDirectory is persistent storage, separate from disposable cache.
func OfficeHistoryDirectory() string {
	if Config.Server.OfficeHistoryDir != "" {
		return Config.Server.OfficeHistoryDir
	}
	return filepath.Join(filepath.Dir(Config.Server.DatabaseV2.Path), ".filebrowser-history")
}
