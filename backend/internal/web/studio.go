package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

const studioMaxBytes = 64 << 20

var studioWrites [64]sync.Mutex

func registerStudioRoutes(router *http.ServeMux, baseURL string) {
	root, err := fs.Sub(assetFs, "studio")
	if err != nil {
		return
	}
	if _, err = fs.Stat(root, "index.html"); err != nil {
		return
	}
	prefix := baseURL + "studio/editor/"
	router.Handle("GET "+prefix, http.StripPrefix(prefix, http.FileServer(http.FS(root))))
}
func studioETag(body []byte) string { return fmt.Sprintf(`"%x"`, sha256.Sum256(body)) }
func studioFile(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	source, name := r.URL.Query().Get("source"), r.URL.Query().Get("path")
	clean, err := utils.SanitizePath(name)
	if err != nil {
		return 400, err
	}
	ext := strings.ToLower(filepath.Ext(clean))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".svg", ".json":
	default:
		return 415, fmt.Errorf("unsupported image or project format")
	}
	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return 403, os.ErrPermission
	}
	idx := indexing.GetIndex(source)
	if idx == nil {
		return 404, os.ErrNotExist
	}
	scope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return 403, os.ErrPermission
	}
	indexed := utils.JoinScopedIndexPath(scope, clean)
	if !state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(indexed, true), d.User.Username) {
		return 403, os.ErrPermission
	}
	// Share the normal PUT resolver's source/scope/symlink/private-path checks.
	physical, err := resolvePutRealPath(idx, indexed)
	if err != nil {
		return sanitizePutPathError(err)
	}
	key := sha256.Sum256([]byte(strings.ToLower(physical)))
	lock := &studioWrites[int(key[0])%len(studioWrites)]
	lock.Lock()
	defer lock.Unlock()
	info, statErr := os.Stat(physical)
	exists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return ErrToStatus(statErr), statErr
	}
	if exists && (info.IsDir() || info.Size() > studioMaxBytes) {
		return 413, fmt.Errorf("image/project must be a file under 64 MiB")
	}
	var before []byte
	if exists {
		// Existing file reads retain FileBrowser's hidden-extension and current access checks.
		if _, err = files.FileInfoFaster(utils.FileOptions{Path: clean, Source: source, FollowSymlinks: true}, d.User); err != nil {
			return ErrToStatus(err), err
		}
		before, err = os.ReadFile(physical)
		if err != nil {
			return ErrToStatus(err), err
		}
	}
	writable := perms.Modify && !idx.Config.ReadOnly
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method == http.MethodGet {
		if !exists {
			return 404, os.ErrNotExist
		}
		w.Header().Set("ETag", studioETag(before))
		w.Header().Set("X-Studio-Writable", fmt.Sprint(writable))
		contentType := "application/octet-stream"
		if ext == ".json" {
			contentType = "application/json"
		} else if ext == ".svg" {
			contentType = "image/svg+xml"
		} else {
			contentType = http.DetectContentType(before)
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// SVG is returned as attachment; the editor decodes bytes in an image element.
		w.Header().Set("Content-Disposition", "attachment")
		_, err = w.Write(before)
		return 0, err
	}
	if idx.Config.ReadOnly || (exists && !perms.Modify) || (!exists && !perms.Create) {
		return 403, os.ErrPermission
	}
	if exists {
		if r.Header.Get("If-Match") != studioETag(before) {
			return 412, fmt.Errorf("file changed; save a copy or reopen the current original")
		}
	}
	if !exists && r.Header.Get("If-None-Match") != "*" {
		return 428, fmt.Errorf("new files require If-None-Match: *")
	}
	if exists && r.Header.Get("If-None-Match") == "*" {
		return 412, fmt.Errorf("a file with that name already exists")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, studioMaxBytes+1))
	if err != nil {
		return 400, err
	}
	if len(body) == 0 || len(body) > studioMaxBytes {
		return 413, fmt.Errorf("image/project must be under 64 MiB")
	}
	detected := http.DetectContentType(body)
	switch ext {
	case ".png":
		if detected != "image/png" {
			return 415, fmt.Errorf("PNG data required")
		}
	case ".jpg", ".jpeg":
		if detected != "image/jpeg" {
			return 415, fmt.Errorf("JPEG data required")
		}
	case ".webp":
		if detected != "image/webp" {
			return 415, fmt.Errorf("WebP data required")
		}
	case ".json":
		var project struct {
			Info   json.RawMessage   `json:"info"`
			Layers []json.RawMessage `json:"layers"`
		}
		if json.Unmarshal(body, &project) != nil || len(project.Info) == 0 || project.Layers == nil {
			return 415, fmt.Errorf("miniPaint layered project required")
		}
	default:
		return 415, fmt.Errorf("save as PNG, JPEG, WebP or a layered JSON project")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("X-File-Size", fmt.Sprint(len(body)))
	r.Header.Set("X-File-Total-Size", fmt.Sprint(len(body)))
	var status int
	w.Header().Set("ETag", studioETag(body))
	if exists {
		status, err = resourcePutHandler(w, r, d)
	} else {
		status, err = ResourcePostHandler(w, r, d)
	}
	if err != nil || status >= 400 {
		return status, err
	}
	return status, nil
}
