package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/officecollab"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var officeSessions = officecollab.New()
var officeOwners sync.Map

type officeRequest struct {
	Room     string            `json:"room"`
	Client   string            `json:"client"`
	After    int64             `json:"after"`
	Revision int64             `json:"revision"`
	Batch    string            `json:"batch"`
	Lease    string            `json:"lease"`
	Changes  []json.RawMessage `json:"changes"`
	Extra    json.RawMessage   `json:"extra"`
	Cursor   string            `json:"cursor"`
	Blocks   []json.RawMessage `json:"blocks"`
}
type officeWriteKey struct{}

func officeFile(r *http.Request, d *Context) (string, bool, error) {
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if d.Share.Hash != "" {
		source = d.Share.GetSourceName()
	}
	clean, err := utils.SanitizePath(path)
	if err != nil {
		return "", false, err
	}
	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return "", false, os.ErrPermission
	}
	scope := ""
	if d.Share.Hash != "" {
		scope = d.Share.Path
	} else {
		scope, err = d.User.GetScopeForSourceName(source)
		if err != nil {
			return "", false, os.ErrPermission
		}
	}
	indexPath := utils.JoinScopedIndexPath(scope, clean)
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", false, os.ErrNotExist
	}
	if !state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(indexPath, true), accessCheckUsername(d)) {
		return "", false, os.ErrPermission
	}
	full, err := filepath.EvalSymlinks(filepath.Join(idx.Path, indexPath))
	if err != nil {
		return "", false, err
	}
	root, err := filepath.EvalSymlinks(idx.Path)
	if err != nil {
		return "", false, err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, os.ErrPermission
	}
	switch strings.ToLower(filepath.Ext(full)) {
	case ".docx", ".xlsx", ".pptx", ".pdf":
	default:
		return "", false, fmt.Errorf("unsupported collaboration format")
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", false, os.ErrNotExist
	}
	return full, perms.Modify && !idx.Config.ReadOnly, nil
}
func officePrincipal(d *Context) string {
	if d.Share.Hash != "" {
		return "share:" + d.Share.Hash
	}
	return "user:" + d.User.Username
}
func officeStatus(err error) int {
	if errors.Is(err, os.ErrPermission) {
		return 403
	}
	if errors.Is(err, os.ErrNotExist) {
		return 404
	}
	if errors.Is(err, officecollab.ErrConflict) || errors.Is(err, officecollab.ErrSession) || strings.Contains(err.Error(), "outside this session") {
		return 409
	}
	return 400
}
func officeHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	path, edit, err := officeFile(r, d)
	if err != nil {
		return officeStatus(err), err
	}
	action := r.PathValue("action")
	if action == "join" {
		officeSessions.SetJournalDir(filepath.Join(settings.Config.Server.CacheDir, "office-sessions"))
		name := "Guest"
		if d.User != nil {
			name = d.User.Username
		}
		s, err := officeSessions.Join(path, name, edit)
		if err != nil {
			return officeStatus(err), err
		}
		officeOwners.Store(s.Room+":"+s.Client, officePrincipal(d))
		if err = officeSessions.BindOwner(path, s.Room, s.Client, officePrincipal(d)); err != nil {
			return 500, err
		}
		return RenderJSON(w, r, s)
	}
	var req officeRequest
	if action == "base" || action == "checkpoint" {
		q := r.URL.Query()
		req.Room = q.Get("room")
		req.Client = q.Get("client")
		req.Lease = q.Get("lease")
		req.Revision, _ = strconv.ParseInt(q.Get("revision"), 10, 64)
	} else {
		if err = json.NewDecoder(io.LimitReader(r.Body, 5<<20)).Decode(&req); err != nil {
			return 400, err
		}
	}
	if action == "resume" {
		officeSessions.SetJournalDir(filepath.Join(settings.Config.Server.CacheDir, "office-sessions"))
		name := "Guest"
		if d.User != nil {
			name = d.User.Username
		}
		s, err := officeSessions.Resume(path, req.Room, req.Client, officePrincipal(d), name, edit, req.After)
		if err != nil {
			return officeStatus(err), err
		}
		officeOwners.Store(req.Room+":"+req.Client, officePrincipal(d))
		return RenderJSON(w, r, s)
	}
	owner, ok := officeOwners.Load(req.Room + ":" + req.Client)
	if !ok || owner != officePrincipal(d) {
		return 403, os.ErrPermission
	}
	if action != "sync" && action != "base" && action != "leave" && !edit {
		return 403, os.ErrPermission
	}
	var result interface{}
	switch action {
	case "base":
		var data []byte
		data, err = officeSessions.Base(path, req.Room, req.Client)
		if err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Cache-Control", "no-store")
			_, err = w.Write(data)
			return 0, err
		}
	case "sync":
		result, err = officeSessions.Sync(path, req.Room, req.Client, req.After)
	case "acquire":
		var lease string
		lease, err = officeSessions.Acquire(path, req.Room, req.Client, req.Revision)
		result = map[string]string{"lease": lease}
	case "changes":
		result, err = officeSessions.Change(path, req.Room, req.Client, req.Batch, req.Lease, req.Changes, req.Extra)
	case "cursor":
		err = officeSessions.Cursor(path, req.Room, req.Client, req.Cursor)
	case "locks":
		err = officeSessions.Locks(path, req.Room, req.Client, req.Blocks)
	case "release":
		err = officeSessions.Release(path, req.Room, req.Client, req.Lease)
	case "leave":
		err = officeSessions.Leave(path, req.Room, req.Client)
		officeOwners.Delete(req.Room + ":" + req.Client)
	case "checkpoint":
		rec := httptest.NewRecorder()
		writeReq := r.WithContext(context.WithValue(r.Context(), officeWriteKey{}, true))
		err = officeSessions.Checkpoint(path, req.Room, req.Client, req.Lease, req.Revision, func() error {
			var status int
			var writeErr error
			if d.Share.Hash != "" {
				status, writeErr = publicPutHandler(rec, writeReq, d)
			} else {
				status, writeErr = resourcePutHandler(rec, writeReq, d)
			}
			if writeErr != nil {
				return writeErr
			}
			if status >= 400 {
				return fmt.Errorf("file save failed (%d)", status)
			}
			return nil
		})
		if err == nil {
			return RenderJSON(w, r, map[string]int64{"saved": req.Revision})
		}
	default:
		return 404, fmt.Errorf("unknown collaboration action")
	}
	if err != nil {
		return officeStatus(err), err
	}
	if result == nil {
		result = map[string]bool{"ok": true}
	}
	return RenderJSON(w, r, result)
}
