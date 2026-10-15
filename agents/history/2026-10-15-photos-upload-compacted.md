# Photos upload recovery and duplicate names

User reported raw Cloudflare502 HTML displayed as an upload error in the actual
C:/tmp/workspace installation. The screenshot timestamp was07:55UTC, during
earlier suite recovery; current services were healthy. User subsequently retried
and confirmed uploads work. Later native logs showed successful uploads and409
for an existing filename. Do not claim an ongoing backend/format fault or prove
a particular original502 cause from the screenshot alone.

User explicitly requested automatic `(xx)` suffixes for duplicates. Photos now
preserves the existing file and tries `photo (1).png`, `photo (2).png`, etc. It
checks candidate metadata before sending bytes, retains the extension, handles
a racing409 by choosing another name/new session/resetting offset, and bounds
the search to1000 alternatives. Original names already ending in `(n)` continue
with the next number. It never enables overwrite automatically.

## Changes

- New frontend/src/utils/httpError.js: bounded messages for gateway/network,
  auth, conflict and limit errors; HTML bodies never rendered as error messages.
  Photos API and normal Drive/private/public upload handlers use it. Native
  Drive upload manager recognises gateway statuses and displays useful details.
- Photos upload API uses XHR progress and Saving phase,120s request timeout,
  stable in-memory per-File/session/confirmed-offset state, strict native
  `{offset,total,complete}` acknowledgement checking, and remaining-file errors.
- PhotosLibrary separates Refresh library from Retry remaining uploads. Pending
  files and original destination survive library refresh. Confirmed completed
  files are omitted from retry; successful uploads report numbered duplicates.
  Closing/reloading discards the in-memory selection. An uncertain final reply
  can produce a numbered copy on retry; docs explain inspecting the library.
- build-photos.ps1 stages the new helper and accepts -Version. PHOTOS.md updated.

## Build boundaries and evidence

No automated or manual upload tests were added/run. User performed the successful
retry on the existing installation. Vite production builds, JS syntax and Go
Windows compilation were used. New duplicate/resume behavior is implemented but
has not had functional acceptance. No personal photos were copied or inspected.

Concurrent unrelated sidebar/design-system/CSS work appeared during this task.
Preserve it and do not stage/publish it with this fix. The first shared-cache
build included some concurrent styles and was discarded. Final build source is
C:/Projects/.cache/photos-upload-build, archived from HEAD47e23a95 and overlaid
only with the seven intended upload/docs/script files. Baseline Office/Studio
embeds were preserved from the build cache. Shared node_modules matched the
lockfile. A same-source staging-script invocation failed; corrected by calling
the repository build-photos script with the isolated build destination.

Final paired binaries under C:/Projects/.cache/photos-upload-final are
FileBrowser.exe and Photos.exe, photos-upload-preview.2. Full core includes
existing Office/Photos/Studio. Suite packaging source separately staged under
C:/Projects/.cache/photos-upload-suite from the previously deployed Mail build.
One Workspace installer target0.3.0-photos-upload-preview.10 retains Mail
0.1.0-subdomain-preview.8 and baseline Accounts/Cloudflared. No publication or
Actions authorized. See one-workspace's matching history for local deployment.

Final naming update supersedes the earlier preview.1 staging targets: both apps
are photos-upload-preview.2. Local ignored binaries/docs/licenses are under
build/photos-upload-preview/. Actual C:/tmp/workspace apps were replaced after
normal manager stop and exact PID/executable checks. Changed EXEs and config
backed up under Private/backups/photos-upload-fix-20261015; these are executable/
configuration backups, not a full user-data backup. Native folder contents,
identity settings, Mail state and DNS were preserved. No GitHub publication.
User must refresh Photos to load the new embedded frontend. Duplicate uploads
were not exercised by the agent; user confirmation predates this deployment.

Deployed runtime completed8/8 startup checks. Photos companion reports
photos-upload-preview.2; its served JS contains the new retry/naming UI. All
suite processes healthy and retained Mail domain has no hosting/identity error.
