# Photos companion and sidebar handoff

## Scope and repository

User requested pulling latest Git and adding a lightweight portable Windows Photos.exe, Google Photos-style browsing of existing Quantum files, and a Photos sidebar section. Follow-up explicitly deferred Android uploads/app, while requesting an architecture suitable for a future Android Photos app. No current-task publication or testing request. No commits, push or release made. Repository I:/Projects/filebrowser-drive, drive-reskin; ff-only pull from fork completed, already current. HEAD a703502 is two local commits ahead of fork/drive-reskin 2f7e461. Preserve unrelated untracked drive-preview.png and frontend/drive-preview.html.

## Implementation

- Shared responsive Vue PhotosLibrary, PhotosIcon and framework-independent API adapter in frontend/src/components/photos/. Timeline grouped by file modification date, filename search, media filters, folder albums, server-persisted favourites using existing pinned items, original viewer/downloads, chunked uploads, keyboard navigation and mobile layout.
- Integrated authenticated /photos route and sidebar link inherit existing FileBrowser sign-in/scopes. currentView recognises Photos before stale file/editor metadata. Office/MyAccounts sources remain unchanged.
- Authenticated /api/photos/capabilities, /library and /media in backend/internal/web/photos.go. Library queries existing index via database/sql/photos.go and pkg/indexing/photos.go; no photo-byte database/import. Context cancellation, literal source/folder prefix, access/hidden/extension filtering, view permissions, bounded pages and encrypted ephemeral cursors bound to account/source/scope/filter. Media uses existing scoped streaming and explicit private/no-store option added to ServeSingleFileOptions; other callers retain existing cache behaviour.
- apps/photos is a separate stdlib Go module with embedded standalone Vue bundle. Loopback-only proxy, Host and mutation Origin checks, existing password authentication/session cookies, no credential persistence. Optional adjacent Photos.json, CLI overrides, Photos.log, browser launch on double click. Defaults FileBrowser 8080, local Photos 8860. SSO shares sessions only with matching hostname; remote SSO-only users use integrated /photos. Existing MyAccounts registrations/secrets not changed.
- Compatibility adapter incrementally scans existing folder APIs on older installations or index-disabled sources; no new backend required for companion. Native sidebar and efficient pagination require upgraded core. Photos.json accepts UTF-8 BOM for hand-edited Windows files; build script writes BOM-free UTF-8.
- PHOTOS.md documents usage, constraints, API v1 and future Android work. scripts/build-photos.ps1 stages/builds standalone frontend and EXE, outputs config/docs/licenses. apps/photos/web assets generated/ignored, README sentinel tracked.

## Build and visual review

Both frontend production builds and CGO_ENABLED=0 Windows Go builds succeeded with Go 1.27.2. No test suites added or run. Desktop integrated/companion and 390px mobile screenshots visually reviewed; actual sample thumbnails displayed. This does not establish functional or security test coverage. Initial isolated preview config incorrectly put source file flags under account.permissions; removed invalid fields and restarted successfully. Existing authenticated previews were not modified.

Build stage: C:/Projects/.cache/filebrowser-office-build. Full backend embed retains ranuts-document Office assets. Final binaries at I:/Projects/tooling/releases/photos-preview/:

- Photos.exe: 7,720,960 bytes, SHA256 cee5b11051327c3a84b9972e4d28b242a42ef62cacb8c79f5a67197da49f52bb.
- filebrowser.exe: 561,566,720 bytes, SHA256 304c32928cdd422f79083038c4189b66de6e7970b7790ecae29c6c43dbdcda23.
- Photos-windows-amd64.zip contains companion/config/docs/licenses; SHA256 de31783eb2823540bc053ff2725abb37ed4660d274fc68dfed8e9a5d8184ad57. Bundled Photos.json targets existing local MyAccounts FileBrowser 8810; CLI overrides for isolated preview.

Version photos-preview, core CommitSHA a703502-photos-working-tree (Photos changes uncommitted). Published v0.4.0 binary remains untouched. Build-stage copies occasionally used wrong relative paths; corrected absolute copies/gofmt before final builds. No unresolved compilation errors.

## Runtime and resume

Isolated preview only: http://127.0.0.1:8830/photos (integrated), http://127.0.0.1:8831/photos (companion). Core PID 78532; companion PID 89632 at handoff; verify owning listener and executable before stopping. Both executable paths above, processes hidden. Runtime I:/Projects/filebrowser-office-preview/photos-runtime, config/database/cache; ordinary sample files under sibling photos-files/Nature and Landscapes copied from Windows wallpapers, dates adjusted only on copies. No-auth enabled only in this isolated loopback preview; do not expose via tunnel. No real-user folder changes. Screenshots sibling photos-integrated.png, photos-companion.png, photos-mobile.png outside Git.

Existing MyAccounts local 8810 and remote 8794, protected launchers/secrets and published Office binaries unchanged. Read 2026-10-14-myaccounts-sso-compacted.md before changing them. Leave unrelated 8787/8788/8798/8799/8811 listeners untouched. No new tunnel or external publication in this task.

Future Android: versioned HTTP contract/shared UI foundation only. No APK, background uploader, device enrolment or Google cloud parity claimed. Native auth (PKCE/revocable folder-scoped device credentials), durable queue/idempotent receipts, MediaStore/WorkManager, EXIF capture dates, virtual albums and trash need separate implementation/verification. Current folder albums are real folders, timeline uses modified dates, decoding depends on browser/thumbnail support. Next release task should request tests, review compatibility/security flows and build script, then explicitly authorize commit/push/release. Keep secrets and preview authentication artifacts out of Git/history.


## MyAccounts login compatibility check

User asked whether MyAccounts login also works with the new Photos feature.
Read-only source inspection confirmed integrated /photos and all three new Photos
API endpoints use existing FileBrowser authentication/permission middleware.
Do not confuse the original no-auth isolated gallery demo with a verified SSO run.

Fresh protected disposable identity using the published MyAccounts
preview-20261014.2 plus the exact photos-preview core/companion binaries passed:
- Real Chrome MyAccounts OIDC ordinary-user login/callback to the photo-enabled core.
- Integrated Photos navigation/library and opening a synthetic PNG.
- Capabilities/library/media returned200 for that signed-in user, directly and
  through Photos.exe; anonymous requests to all three returned401.
- Same-host localhost Photos.exe automatically reused the FileBrowser cookie.
- Requesting the companion at127.0.0.1 instead returned401, proving that hostname
  alignment is required. Different ports are fine; localhost and127.0.0.1 differ.
- Native base health/admin/bootstrap/clean stop/backup/restore/signer smoke passed.
This verifies the tested login/photo access flow, not uploads or every permissions
case/full Photos acceptance. Only synthetic data was used; no live identities,
photo folders or existing preview configurations changed.

Current packaged Photos.json still uses server127.0.0.1:8810/listen127.0.0.1:8860,
while existing MyAccounts FileBrowser callbacks use localhost. For sharing that
existing local SSO session, launch with --server http://localhost:8810
--listen localhost:8860 (or configure both hostnames consistently). Remote
SSO-only servers should use their integrated /photos because their hostname
cannot share a host-only browser cookie with a local companion.

Existing authenticated8810/8794 processes still use drive-office-v0.4.0, not the
new photos-preview core; this check did not upgrade them. Photos feature source
remains uncommitted. No commit/push/release requested or performed for this login
question. Disposable checker scripts are ignored in the MyAccounts dist directory.


## Public integration preview started

User explicitly requested tunnelling all three apps to test integration. New
protected F:/MyAccounts/integration-preview runtime serves photos-preview core on
8850, Photos.exe on8851, same-origin gateway8852 and an isolated Quick Tunnel.
Read its connection.json/README.txt for current temporary links and processes.json
for owned process identities; do not commit the temporary hostname or credentials.
MyAccounts uses existing public login.m8e.co.uk. FileBrowser root and standalone
Photos /photos-app/ share the Quick Tunnel origin and Secure/HttpOnly session;
integrated Photos /photos is also available. All /api traffic goes to FileBrowser.
Photos HTML/static are served from actual companion executable; public companion
config points to public FileBrowser rather than localhost. This avoids cross-host
session problems and prevents the companion's local proxy rewriting public cookies.

New core has fresh settings DB and recovery administrator, authenticated OIDC and
noauth=false, default admin/API false. Uses existing sample photos-files library.
New separate exact MyAccounts callback/client; protected DPAPI secret in F runtime.
Original no-auth demo8830/8831 and existing authenticated8810/8794 left alone.
Never point the public tunnel at the original no-auth demo.

Public Chrome proof passed Photos launch -> FileBrowser -> MyAccounts -> callback,
normal non-admin session, integrated library/original photo, standalone resume and
reload without another login. Anonymous profile/library/media denied401, another
admin profile403, public cookie Secure/HttpOnly. Gateway Host/Origin boundaries set;
streaming/WebSocket forwarding uses Go ReverseProxy. Full upload/Office regressions
were not repeated. See companion MyAccounts history for routing diagnosis: explicit
Quick Tunnel config avoided inherited shared Cloudflare config's catchall404.
No shared CF config/DNS modified and no source commit/push/release requested here.
