<img src="frontend/src/assets/branding/my_photos.svg" alt="My Photos" width="80" height="80">

# Photos for FileBrowser Drive

Photos is a Google Photos-style view of the photos and videos in your existing FileBrowser folders. It includes a dated thumbnail timeline, filename search, photo/video filters, folder albums, favourites, original downloads and a keyboard-accessible fullscreen viewer. The timeline currently uses **file modification dates**, not EXIF capture dates. Favourites use FileBrowser's existing per-account pinned items, so they persist on the server. Albums are ordinary folders, not imported copies.

## In the FileBrowser sidebar

Build/install the updated FileBrowser frontend/backend, then choose **Photos** in the sidebar or open `/photos`. It inherits FileBrowser's current sign-in, including MyAccounts, source scopes, access rules, view/download/create permissions and quotas. Existing Office editing and history remain part of that executable.

## Portable Windows companion

`Photos.exe` embeds only the small Photos frontend and a Go HTTP proxy. It connects to an already-running FileBrowser; it does not start another file store, scan Windows folders independently, or require Docker, Node.js or Go at runtime. A desktop browser is required.

```powershell
.\Photos.exe --server http://127.0.0.1:8810
```

The companion opens `http://127.0.0.1:8860/photos`. By default it targets FileBrowser on port 8080. An optional `Photos.json` beside the executable supplies defaults for double-click launch:

```json
{"server":"http://127.0.0.1:8810","listen":"127.0.0.1:8860"}
```

CLI options override the file. `--listen 127.0.0.1:8861` selects another local port, and `--no-browser` keeps the browser closed on startup. The listener is restricted to loopback. FileBrowser must stay running. The companion creates no document database and stores no account passwords; FileBrowser owns the authenticated session. `Photos.log` beside the executable records startup errors.

Password sign-in works through the authenticated upstream APIs. For MyAccounts, passkeys or two-step sign-in, use **FileBrowser sign-in / MyAccounts**, complete sign-in in that tab, return to Photos and choose **I'm signed in**. This shares an existing session only when the companion and the FileBrowser callback use the same hostname (for example both `127.0.0.1`). For an SSO-only remote server on a different hostname, use its integrated `/photos` section. No MyAccounts clients or callback registrations are changed by Photos.

Older Quantum installations can use the companion through the existing folder, preview, view-grant, upload and pinned-item APIs. The compatibility mode scans folders incrementally; newer installations use the efficient paginated photo index. Update the core executable to add the built-in sidebar section and the new API. Very large older libraries benefit from upgrading the core. Supported image/video decoding follows the browser and FileBrowser's available thumbnail generators; formats such as HEIC and some video codecs may require another viewer. Preview failures retain an Open in FileBrowser action. Original files are never converted or replaced by the gallery.

Uploads use FileBrowser's normal chunked upload endpoint. Duplicate filenames produce a conflict rather than overwriting existing files. Create permissions and quotas still apply.

## Build Photos.exe

Prerequisites: Node.js compatible with `frontend/package.json`, npm, Go 1.27+, and Windows. The script stages source on the local drive to avoid mapped-drive link problems.

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-photos.ps1 -ServerUrl http://127.0.0.1:8810
```

Output: `build/photos/Photos.exe`, optional `Photos.json`, documentation and license notices. The standalone bundle does not include the Office engine. Build the full FileBrowser using the existing `OFFICE.md` instructions to add the sidebar section. No test suite is run by the Photos build script.

## Future Android application

Android automatic uploads are deferred. There is no Android app or background uploader in this version. The frontend is split into a framework-independent API adapter and a reusable responsive Vue component; it does not depend on FileBrowser's router or store. A future native Android app can use the same HTTP contract directly, without keeping a Windows browser open.

The next Android phase should add an OIDC public client with PKCE or explicit device enrolment, revocable upload-only credentials scoped to a chosen folder, a durable local upload queue, checksummed/idempotent upload receipts, MediaStore camera discovery, WorkManager constraints/retry and a permission-aware media cache. Media permissions, selected-photo access, reboot recovery, battery limits and background timing need real Android implementation and verification. Windows files remain the source of truth; thumbnails and mobile metadata caches must be rebuildable. EXIF capture dates, virtual albums, cross-device album changes and deletion/trash semantics should be versioned additions to this API, not frontend-only databases or promises of Google cloud features.

### Photos API v1

All endpoints use FileBrowser's existing authenticated session or supported API bearer tokens, and recheck current source permissions. Existing account policy controls API token creation.

| Endpoint | Contract |
| --- | --- |
| `GET /api/photos/capabilities` | `version:1`, opaque cursor pagination, file-date basis, ordinary file storage, upload/pinned-item integrations |
| `GET /api/photos/library` | `source` required; `path=/`, `q`, `kind=image\|video`, `favorites=true`, `limit=1..200`, opaque `cursor` optional |
| Library response | `{version:1, items:[{id,source,path,name,size,modified,type,kind,hasPreview,favorite,permissions}], nextCursor, dateBasis:"modified"}` |
| `GET /api/photos/media?source=…&path=…` | Authenticated original image/video bytes, inline range support, no download accounting; current view permission required |
| `GET /api/resources/items` | Browse permitted folders for folder albums |
| `PATCH /api/users/pinned-items?action=add\|remove` | Body `{source,path:<parent folder>,name:<basename>}`; per-account favourites |
| `POST /api/resources?source=…&path=…` | Existing quota-aware chunked file upload; `X-File-Chunk-Offset`, `X-File-Total-Size`, `X-File-Upload-Session`; response `{offset,total,complete}` |

Media IDs derive from source and scope-relative path; a rename changes the ID. Cursors are opaque, authenticated, bound to the user/source/folder/filter, and ephemeral across a server restart. On an expired-cursor response, reload the first page. Paging uses modified time and path as a stable tie-breaker; concurrent filesystem edits can move items between pages, so clients deduplicate by ID and expose Refresh. Indexed media can take a normal source scan to appear after external Windows changes. History/cache directories and denied/hidden files remain excluded. Photo bytes are never inserted into a proprietary media database.
