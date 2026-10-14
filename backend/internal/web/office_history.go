package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/officehistory"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var officeVersions = officehistory.New("", 0)

func configureOfficeHistory() {
	officeVersions.Configure(settings.OfficeHistoryDirectory(), settings.Config.Server.OfficeHistoryLimit)
}
func officeAuthor(d *Context) string {
	if d.User != nil {
		return d.User.Username
	}
	return "Guest"
}
func officeWriteBytes(r *http.Request, d *Context, body []byte) error {
	recorder := httptest.NewRecorder()
	request := r.Clone(context.WithValue(r.Context(), officeWriteKey{}, true))
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.Header = request.Header.Clone()
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Del("X-File-Chunk-Offset")
	request.Header.Set("X-File-Total-Size", fmt.Sprint(len(body)))
	request.Header.Set("X-File-Size", fmt.Sprint(len(body)))
	var status int
	var err error
	if d.Share.Hash != "" {
		status, err = publicPutHandler(recorder, request, d)
	} else {
		status, err = resourcePutHandler(recorder, request, d)
	}
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("file save failed (%d)", status)
	}
	return nil
}
func officeHistoryAction(w http.ResponseWriter, r *http.Request, d *Context, path string, edit bool) (int, error) {
	// A current editor role is required even for previews/downloads; viewing the
	// current document does not disclose historical, possibly removed content.
	if !edit {
		return http.StatusForbidden, fmt.Errorf("edit permission is required to view version history")
	}
	configureOfficeHistory()
	action := r.PathValue("action")
	if action == "history-blob" {
		v, body, err := officeVersions.Read(path, r.URL.Query().Get("version"), r.URL.Query().Get("digest"))
		if err != nil {
			return officeStatus(err), err
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", mime.TypeByExtension(v.Format))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + v.Format}))
		http.ServeContent(w, r, filepath.Base(path), v.Created, bytes.NewReader(body))
		return 0, nil
	}
	var req struct {
		Version  string `json:"version"`
		Digest   string `json:"digest"`
		Name     string `json:"name"`
		Room     string `json:"room"`
		Client   string `json:"client"`
		Lease    string `json:"lease"`
		Revision int64  `json:"revision"`
	}
	if r.Method != "POST" {
		return 405, fmt.Errorf("use POST for history actions")
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&req); err != nil {
		return 400, err
	}
	switch action {
	case "history-list":
		versions, err := officeVersions.List(path)
		if err != nil {
			return officeStatus(err), err
		}
		return RenderJSON(w, r, map[string]any{"versions": versions})
	case "history-name":
		v, err := officeVersions.Name(path, req.Version, req.Digest, req.Name)
		if err != nil {
			return officeStatus(err), err
		}
		return RenderJSON(w, r, v)
	case "history-restore":
		owner, ok := officeOwners.Load(req.Room + ":" + req.Client)
		if !ok || owner != officePrincipal(d) {
			return 403, fmt.Errorf("this document session belongs to another user")
		}
		err := officeSessions.Restore(path, req.Room, req.Client, req.Lease, req.Revision, func(current []byte) error {
			commitErr := officeVersions.Commit(path, current, nil, []string{officeAuthor(d)}, req.Room, req.Revision, req.Version, req.Digest, func(body []byte) error { return officeWriteBytes(r, d, body) })
			if errors.Is(commitErr, officehistory.ErrDeferred) {
				return nil
			}
			return commitErr
		})
		if err != nil {
			return officeStatus(err), err
		}
		return RenderJSON(w, r, map[string]bool{"restored": true})
	default:
		return 404, fmt.Errorf("unknown history action")
	}
}
func isOfficeDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx", ".xlsx", ".pptx", ".pdf":
		return true
	}
	return false
}
func officeCreated(path string, d *Context) {
	if !isOfficeDocument(path) {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > officehistory.MaxDocumentBytes {
		return
	}
	configureOfficeHistory()
	if err = officeVersions.Created(path, officeAuthor(d)); err != nil { /* Join will retry capturing this ordinary file and report a storage error. */
	}
}

// Capture an existing Office file before an upload replaces it. A later List
// records the uploaded current file as a change outside the live editor.
func officeBeforeReplace(path string) error {
	if !isOfficeDocument(path) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > officehistory.MaxDocumentBytes {
		return nil
	}
	configureOfficeHistory()
	_, err = officeVersions.List(path)
	return err
}
func putResourceWithHistory(w http.ResponseWriter, r *http.Request, d *Context, source, indexPath string) (int, error) {
	if r.Context().Value(officeWriteKey{}) == true || !isOfficeDocument(indexPath) {
		return putResourceWithQuota(w, r, d, source, indexPath)
	}
	path, _, err := officeFile(r, d)
	if err != nil {
		return putResourceWithQuota(w, r, d, source, indexPath)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > officehistory.MaxDocumentBytes || r.ContentLength > officehistory.MaxDocumentBytes {
		return putResourceWithQuota(w, r, d, source, indexPath)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return 500, err
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, officehistory.MaxDocumentBytes+1))
	if err != nil {
		return 400, err
	}
	if len(body) > officehistory.MaxDocumentBytes {
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
		return putResourceWithQuota(w, r, d, source, indexPath)
	}
	configureOfficeHistory()
	status := 0
	err = officeVersions.Commit(path, before, body, []string{officeAuthor(d)}, "filesystem", 0, "", "", func(data []byte) error {
		r.Body = io.NopCloser(bytes.NewReader(data))
		var e error
		status, e = putResourceWithQuota(w, r, d, source, indexPath)
		if e != nil {
			return e
		}
		if status >= 400 {
			return fmt.Errorf("file save failed (%d)", status)
		}
		return nil
	})
	if errors.Is(err, officehistory.ErrDeferred) {
		err = nil
	}
	if err != nil && status < 400 {
		status = officeStatus(err)
	}
	return status, err
}
func officeMove(from, to string, isDir bool, write func() error) error {
	if officeSessions.ActiveUnder(from, isDir) || officeSessions.ActiveUnder(to, isDir) {
		return fmt.Errorf("close live Office documents before moving or renaming their files or folders")
	}
	configureOfficeHistory()
	return officeVersions.Move(from, to, isDir, write)
}
