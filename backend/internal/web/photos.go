package web

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

type photoCursor struct {
	Time int64  `json:"time"`
	Path string `json:"path"`
}

var photoPages = cipherForPhotoPages()

func cipherForPhotoPages() cipher.AEAD {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return aead
}
func photosCapabilities(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	return RenderJSON(w, r, map[string]any{"version": 1, "pagination": "opaque-cursor", "dateBasis": "modified", "storage": "ordinary-files", "uploads": "resources", "favorites": "pinned-items"})
}
func photosLibrary(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	q := r.URL.Query()
	source := q.Get("source")
	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return 403, fmt.Errorf("view permission is required")
	}
	idx := indexing.GetIndex(source)
	if idx == nil {
		return 404, fmt.Errorf("source not found")
	}
	if idx.IndexingDisabled {
		return 501, fmt.Errorf("indexed Photos is unavailable for this source")
	}
	scope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return 403, err
	}
	folder := q.Get("path")
	if folder == "" {
		folder = "/"
	}
	folder, err = utils.SanitizePath(folder)
	if err != nil {
		return 400, err
	}
	folder = "/" + strings.TrimLeft(folder, "/")
	indexFolder := utils.JoinScopedIndexPath(scope, folder)
	prefix := idx.MakeIndexPath(indexFolder, true).String()
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(indexFolder, true), d.User.Username) {
		return 403, fmt.Errorf("access denied")
	}
	kind := q.Get("kind")
	if kind != "" && kind != "image" && kind != "video" {
		return 400, fmt.Errorf("kind must be image or video")
	}
	query := strings.TrimSpace(q.Get("q"))
	if len(query) > 200 {
		return 400, fmt.Errorf("search must be 200 bytes or fewer")
	}
	limit := 100
	if q.Get("limit") != "" {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			return 400, fmt.Errorf("limit must be between 1 and 200")
		}
	}
	onlyFavorites := q.Get("favorites") == "true"
	favoritePaths := []string{}
	for dir, names := range d.User.PinnedItems[idx.Path] {
		for _, name := range names {
			favoritePaths = append(favoritePaths, path.Join(dir, name))
		}
	}
	if len(favoritePaths) > 10000 {
		return 400, fmt.Errorf("too many pinned items")
	}
	// Authenticated encryption keeps skipped private filenames out of cursors.
	binding, _ := json.Marshal([]any{d.User.ID, source, scope, folder, kind, query, onlyFavorites})
	cursor := photoCursor{}
	if encoded := q.Get("cursor"); encoded != "" {
		raw, e := base64.RawURLEncoding.DecodeString(encoded)
		if e != nil || len(raw) < photoPages.NonceSize() {
			return 400, fmt.Errorf("invalid cursor; refresh Photos")
		}
		body, e := photoPages.Open(nil, raw[:photoPages.NonceSize()], raw[photoPages.NonceSize():], binding)
		if e != nil || json.Unmarshal(body, &cursor) != nil {
			return 400, fmt.Errorf("expired cursor; refresh Photos")
		}
	}
	items := []map[string]any{}
	more := false
	scanned := 0
	for len(items) < limit && scanned < 10000 {
		rows, e := idx.Photos(r.Context(), prefix, query, kind, cursor.Time, cursor.Path, 200, favoritePaths, onlyFavorites)
		if e != nil {
			return 500, e
		}
		if len(rows) == 0 {
			more = false
			break
		}
		consumed := 0
		for _, v := range rows {
			cursor = photoCursor{v.ModTime, v.Path}
			consumed++
			scanned++
			if !state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(v.Path, false), d.User.Username) || utils.HideFileByExt(v.Name, d.User.HideFileExt) {
				continue
			}
			hidden := false
			for _, segment := range strings.Split(v.Path, "/") {
				if strings.HasPrefix(segment, ".") {
					hidden = true
					break
				}
			}
			if hidden && !d.User.ShowHidden {
				continue
			}
			relative := "/" + strings.TrimLeft(strings.TrimPrefix(v.Path, strings.TrimRight(scope, "/")), "/")
			mediaKind := "image"
			if strings.HasPrefix(v.Type, "video/") {
				mediaKind = "video"
			}
			items = append(items, map[string]any{"id": base64.RawURLEncoding.EncodeToString([]byte(source + "\x00" + relative)), "source": source, "path": relative, "name": v.Name, "size": v.Size, "modified": time.Unix(v.ModTime, 0).UTC().Format(time.RFC3339), "type": v.Type, "kind": mediaKind, "hasPreview": v.HasPreview, "favorite": slices.Contains(favoritePaths, v.Path), "permissions": perms})
			if len(items) == limit {
				break
			}
		}
		more = consumed < len(rows) || len(rows) == 200
		if !more {
			break
		}
	}
	next := ""
	if more {
		body, _ := json.Marshal(cursor)
		nonce := make([]byte, photoPages.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return 500, err
		}
		next = base64.RawURLEncoding.EncodeToString(photoPages.Seal(nonce, nonce, body, binding))
	}
	w.Header().Set("Cache-Control", "no-store")
	return RenderJSON(w, r, map[string]any{"version": 1, "items": items, "nextCursor": next, "dateBasis": "modified"})
}
func photosMedia(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	source := r.URL.Query().Get("source")
	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return 403, fmt.Errorf("view permission is required")
	}
	name := r.URL.Query().Get("path")
	info, err := files.FileInfoFaster(utils.FileOptions{Path: name, Source: source, FollowSymlinks: true}, d.User)
	if err != nil {
		return ErrToStatus(err), err
	}
	if info.IsDir || (!strings.HasPrefix(info.Type, "image/") && !strings.HasPrefix(info.Type, "video/")) {
		return 415, fmt.Errorf("not a photo or video")
	}
	scope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return 403, err
	}
	clean, err := utils.SanitizePath(name)
	if err != nil {
		return 400, err
	}
	return ServeSingleFile(w, r, d, source, utils.JoinScopedIndexPath(scope, clean), info.Name, ServeSingleFileOptions{ForceInline: true, RangeOnly: false, NoStore: true})
}
