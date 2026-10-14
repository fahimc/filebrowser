# Live Office collaboration plan

## Intended behavior

Devices opening the same physical file join one session. Edits merge in the open editor, without a file reload. Participants and remote cursor/selection updates identify active devices. Autosave writes the merged document back to its ordinary DOCX, XLSX, PPTX or supported PDF file. The Go executable hosts both the editor and collaboration endpoints.

## Implementation

1. Use the embedded ONLYOFFICE SDK's native incremental changes, position transforms, locks and collaborative undo. Give every browser session a unique object-ID namespace. Load an immutable session baseline and replay ordered changes for late arrivals.
2. Add authenticated Go session endpoints with heartbeat/presence, ordered change batches, idempotent retries and save leases. Recheck source/share permissions for each request. Use ordinary HTTPS polling for transport so the existing Cloudflare Quick Tunnel can carry collaboration without SSE.
3. Coordinate file checkpoints with a revision and a content fingerprint. Reject stale snapshots and external-file conflicts. Keep unacknowledged edits in the open browser during disconnection; retry them on reconnect. Closing waits for synchronization and a successful file save.

## Release checks

- Two independent sessions editing concurrently, including the same paragraph/cell/slide object.
- Content convergence, preservation of both users' changes and ordinary-file save/reopen.
- Late join, reconnect, retry without duplicated changes, and safe stale-checkpoint rejection.
- Participant/cursor updates and permission denial for view-only sessions.
- Existing Office creation and autosave regression checks; executable-only Windows runtime and remote Cloudflare check.

This plan describes the intended implementation. Release notes must report the behavior actually verified, and distinguish any remaining differences from Google Docs.
