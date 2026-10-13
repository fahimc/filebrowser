# Browser Office integration

Open DOCX, XLSX, PPTX, or supported PDF files from FileBrowser. Changes are exported by the browser editor and written to the original file through FileBrowser's authenticated resource API. The status becomes **All changes saved** only after the filesystem write succeeds. A failed write keeps the document open and enables Save for a retry. Closing or navigating away waits for pending edits to save.

Existing user preferences are preserved. If a user's disabled Office extensions include `.pdf`, remove that extension in the file-viewer preferences to enable browser PDF editing.

To create a document, choose **New → New file → DOCX, XLSX, PPTX, or PDF**, enter a filename, and save. Subsequent edits save automatically. PDF support includes the editor's supported annotations and forms; it does not guarantee editing arbitrary PDF page text. Files retain their ordinary formats in the configured Windows folders. FileBrowser's user/settings database does not contain document contents.

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
```

The sample generator uses `python-docx`, `openpyxl`, `python-pptx`, and `reportlab`, only for test fixtures. It leaves existing files intact.

Use a disposable folder: this check edits the four named samples and creates new documents. Backend asset-routing checks also cover root URLs, a configured base URL, hashed CSS, WebAssembly transport headers, and HEAD requests.

## Source and notices

The editor is `ranuts/document` at the pinned submodule commit. The downstream patch enables embedded new-document creation without editor-local document history, reports creation readiness, disables its root-scoped service worker, and maps Calibri's styled faces to the corresponding open Carlito faces. Upstream legal notices remain accessible from the editor's About pane.

See [third-party notices](third_party/NOTICES.md). Releases include the full source archive with the pinned submodule and downstream patch, plus the applicable license and notice files.
