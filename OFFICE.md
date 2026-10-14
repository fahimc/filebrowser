# Browser Office integration

Open DOCX, XLSX, PPTX, or supported PDF files from FileBrowser. Changes are exported by the browser editor and written to the original file through FileBrowser's authenticated resource API. The status becomes **All changes saved** only after the filesystem write succeeds. A failed write keeps the document open and enables Save for a retry. Closing or navigating away waits for pending edits to save.

Existing user preferences are preserved. If a user's disabled Office extensions include `.pdf`, remove that extension in the file-viewer preferences to enable browser PDF editing.

To create a document, choose **New → New file → DOCX, XLSX, PPTX, or PDF**, enter a filename, and save. Subsequent edits save automatically. PDF support includes the editor's supported annotations and forms; it does not guarantee editing arbitrary PDF page text. Files retain their ordinary formats in the configured Windows folders. FileBrowser's user/settings database does not contain document contents.

## Live editing across devices

Open the same file from two browsers using the same server URL. Each device joins a live session: native editor operations merge into the other open editor, participants appear in the toolbar, and supported cursor/selection updates identify other devices. Word streams ongoing typing; spreadsheets commit a cell when you finish editing it. Spreadsheet ranges and slide/PDF objects use editing locks where the SDK requests them.

Autosave checkpoints the merged revision to the original ordinary file. Late arrivals replay the session's changes. Brief network disconnections retain edits in the open tab and retry after reconnect; acknowledged changes have a private recovery journal that lets existing tabs resume after a server restart. The journal is temporary recovery data in `cache/office-sessions`, protected from FileBrowser file access, and must stay with the portable settings/cache when moving an active installation.

This is Google Docs-style live editing using the bundled ONLYOFFICE engine. It does not provide Google Docs' full offline application, comments/history service, or Google account integration. Keep disconnected tabs open: unacknowledged edits are held in browser memory. **Download copy** preserves a local draft when a session expires or an external program changes the original file; conflicting filesystem writes are rejected. PDF support remains limited to the engine's supported annotations/forms. Supported session limits are 100 MiB per initial document, 64 devices, 128 active documents, and 32 MiB of incremental changes per session. Save and close all tabs periodically to start a fresh session.

The collaboration coordinator is part of the same Go executable. HTTPS polling works through the Cloudflare Quick Tunnel without SSE or a separate document server. All devices must connect to the same running FileBrowser instance, which must remain online.

## Portable Windows build

The executable embeds the FileBrowser interface, editor JavaScript, fonts, and WebAssembly engine. Users need only the executable and a browser. Node.js, pnpm, Go, and Git Bash are build prerequisites, with no separate Office server or Office installation required at runtime. The editor bundle makes this executable substantially larger than a FileBrowser-only build.

Clone with recursive submodules. On Windows, the helper builds the pinned editor source and the committed downstream patch in a local staging folder; this avoids pnpm link failures on mapped/network drives.

```powershell
git submodule update --init --recursive
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-office-editor.ps1
cd frontend
npm ci
npm run build:docker
cd ..
New-Item -ItemType Directory -Force backend/internal/web/embed
Copy-Item backend/internal/web/dist/* backend/internal/web/embed -Recurse -Force
bash scripts/copy-ranuts-document.sh
cd backend
$env:CGO_ENABLED = '0'
go build -o filebrowser.exe .
```

Builds on Linux/macOS and the release workflow use `make build`, with pnpm 11.4.0. The optional MuPDF thumbnail renderer is separate from this browser PDF editor; the portable Windows build works without MuPDF.

## Verification

The integrated browser check uses real, disposable sample files and checks Office edits on disk, reopening, failed-save retry, saving before close, PDF annotations, and creation of all four formats. It requires the frontend development dependencies, a current Node.js version with TypeScript stripping, Chrome, and a running FileBrowser configured with the sample folder as a source named `My Drive`.

```powershell
python scripts/create-office-fixtures.py I:/Projects/filebrowser-office-preview/files
node scripts/office-smoke.mjs http://127.0.0.1:8791 I:/Projects/filebrowser-office-preview/files I:/Projects/filebrowser-office-preview/screenshots
node scripts/office-collaboration-smoke.mjs http://127.0.0.1:8791 I:/Projects/filebrowser-office-preview/files I:/Projects/filebrowser-office-preview/collaboration-screenshots
```

The sample generator uses `python-docx`, `openpyxl`, `python-pptx`, and `reportlab`, only for test fixtures. It leaves existing files intact.

Use a disposable folder: this check edits the four named samples and creates new documents. Backend asset-routing checks also cover root URLs, a configured base URL, hashed CSS, WebAssembly transport headers, and HEAD requests.

## Source and notices

The editor is `ranuts/document` at the pinned submodule commit. The downstream patches enable embedded new-document creation without editor-local document history, report creation readiness, disable its root-scoped service worker, map Calibri's styled faces to the corresponding open Carlito faces, and preserve unsent native history when exporting checkpoints or draft copies. Upstream legal notices remain accessible from the editor's About pane.

See [third-party notices](third_party/NOTICES.md). Releases include the full source archive with the pinned submodule and downstream patch, plus the applicable license and notice files.
