# Drive upload confirmation and pause states

User reported small PNG uploads showing100% with Resume controls and apparently
random pauses halfway through a batch. Follow repository-history; no publication
authorized and no tests added/run for this request. Preserve concurrent Drive
design-system/sidebar changes and the earlier Photos upload changes.

## Findings and implementation

The old manager used XHR body-transfer progress as file completion percentage,
then auto-paused after10s without progress, including time spent awaiting the
server's final response. Async chunk pause signalling could overwrite completed
state. Screenshot alone cannot establish the original network interruption;
available runtime logs did not contain a matching failed request. No recent
upload partials were present before local update.

- resources.js reports body-sent separately through the existing progress
  callback's optional second argument. Private/public upload paths both support
  this. Pause signalling accepts a bounded AbortSignal.
- upload.js caps provisional progress99%, uses60s transfer inactivity and a
  separate120s server-confirmation deadline, and only completes after HTTP
  success. Chunk replies must match `{offset,total,complete}` before advancing
  the confirmed resume offset. Whole-file native success has an empty response;
  HTML success pages are rejected as uncertain confirmation.
- Upload.vue visibly shows Waiting for server confirmation, pause/error reasons,
  and next action beside each file. Individual pause is disabled during saving.
  Unknown final replies advise checking the destination before retrying.
- Pause completion wins over a delayed pause signal. Scheduling excludes jobs
  whose prior request is still settling; cancelled jobs cannot be resurrected.
  Pause All lets an in-flight saving chunk finish before stopping at its boundary.
  Paused batches remain visible and do not trigger completion reload. Fixed the
  upload dialog's incorrectly registered visibility handler for wake-lock renewal.

## Build and deployment

Production Vite/checker compilation, JS syntax, whitespace checks and Go Windows
compilation passed. No manual or automated upload tests and no reproduction
claim. Build source C:/Projects/.cache/photos-upload-build was already isolated
from concurrent styling edits; overlaid only upload.js/resources.js/Upload.vue
on the prior intended Photos patch. Office/Studio embedded assets retained.

Core FileBrowser.exe is drive-upload-preview.3, local ignored copy under
build/drive-upload-preview/. Photos companion remains photos-upload-preview.2.
Installer/manager0.3.0-drive-upload-preview.11 packaged in isolated existing
suite source C:/Projects/.cache/photos-upload-suite. See matching one-workspace
history for installer and runtime details. No GitHub/Actions/commit activity.

Actual C:/tmp/workspace updated after exact runtime path verification and normal
manager stop. Changed executables/config backed up under
Private/backups/drive-upload-fix-20261015-100853 (not a full data backup).
New manager started hidden with explicit run, initial PID58868. User must refresh
Drive after uploads finish to load new assets; refreshing discards the current
in-memory file selection. Inspect destination before retrying old uncertain rows.
Existing identity, files, Mail state and DNS preserved.

Deployment readback: startup ready8/8; Drive, Photos and Mail health endpoints
return200. Accounts raw-IP Host request is403 by its Host policy; the same health
request with its configured Host returns200 (public-accounts gate passed). This is
startup/asset evidence only, not functional upload acceptance.
