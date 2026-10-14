# Live Office collaboration plan

## Intended behavior

Devices opening the same physical file join one session. Edits merge in the open editor, without a file reload. Participants and remote cursor/selection updates identify active devices. Autosave writes the merged document back to its ordinary DOCX, XLSX, PPTX or supported PDF file. The Go executable hosts both the editor and collaboration endpoints.

## Implementation

1. Use the embedded ONLYOFFICE SDK's native incremental changes, position transforms, locks and collaborative undo. Give every browser session a unique object-ID namespace. Load an immutable session baseline and replay ordered changes for late arrivals.
2. Add authenticated Go session endpoints with heartbeat/presence, ordered change batches, idempotent retries and save leases. Recheck source/share permissions for each request. Use ordinary HTTPS polling for transport so the existing Cloudflare Quick Tunnel can carry collaboration without SSE.
3. Coordinate file checkpoints with a revision and a content fingerprint. Reject stale snapshots and external-file conflicts. Keep unacknowledged edits in the open browser during disconnection; retry them on reconnect. Closing waits for synchronization and a successful file save.

## Document history

Durable file snapshots accompany successful merged checkpoints. Adjacent autosaves in the same session form five-minute groups, with contributing authors captured when native changes are accepted. Named versions are pinned. JSON indexes and ordinary Office snapshots live beside the settings database, separately from the temporary coediting recovery journal. A prepared metadata entry and both file snapshots precede the normal quota-aware file write; pending entries recover by comparing the current file digest after a crash.

Restoring requires an editor's current save lease, a fully saved room revision and an unchanged source file. It preserves the current snapshot, writes the selected bytes through FileBrowser's existing quota/permission path, retires the old room and records a durable reset marker. Clean peers reload; peers with unsaved native changes retain a downloadable draft. The preview's digest must still match before naming or restoring a grouped version. FileBrowser moves keep history identity and are blocked while affected Office sessions have open devices.

## Release checks

- Two independent sessions editing concurrently, including the same paragraph/cell/slide object.
- Content convergence, preservation of both users' changes and ordinary-file save/reopen.
- Late join, reconnect, retry without duplicated changes, and safe stale-checkpoint rejection.
- Participant/cursor updates and permission denial for view-only sessions.
- Existing Office creation and autosave regression checks; executable-only Windows runtime and remote Cloudflare check.
- All four formats: named version, read-only preview, exact snapshot download, restore, clean peer reload and persistent history; offline draft protection, private history access and crash recovery.

This plan describes the intended implementation. Release notes must report the behavior actually verified, and distinguish any remaining differences from Google Docs.
