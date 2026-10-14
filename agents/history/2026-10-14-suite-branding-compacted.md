# Suite branding

Seven supplied suite logos are canonical in sibling one-workspace/internal/branding/icons.
Drive uses my_drive for header and default login/favicon; custom configured icons remain
supported. my_office is in the built-in Office toolbar. Shared PhotosIcon now loads
my_photos externally for integrated/standalone/header/sidebar; standalone HTML favicon
and login sizing match. Studio sidebar uses photo_editor. frontend/public/index.html
no longer incorrectly declares every configurable favicon as PNG.

New Windows amd64 resource files apply Drive and Photos native icons; build-photos.ps1
stages shared SVGs and the native resource. Preserve all pre-existing uncommitted
Photos/Studio backend and UI work; no unrelated files reverted, staged or published.
Drive and Photos production Vite builds passed; Photos native EXE rebuilt in sibling
one-workspace/dist/branding. Full core native compilation passed, retaining existing Office/Photos/Studio embeds.
Fresh loopback19972 preview with synthetic files rendered the actual Drive header,
Photos header and Photos/Studio sidebar logos correctly.
No new functional tests for artwork-only changes, no live installation changed.
Keep original preview executables/configurations and published assets untouched.

## Source publication follow-up

User requested pushing the affected repositories and adding logos to their main
READMEs. This supersedes the earlier local-only publication scope. All five main
READMEs now reference their local SVG assets. Drive publication includes the
previous Photos/Studio prerequisite source so the branded app imports build from
the pushed branch; unrelated preview files and concurrent Workspace Mail changes
remain excluded. Existing executable releases and live installations are unchanged.
GitHub Actions is disabled before publication; builds remain local.

Source and main README branding pushed and remote SHA verified on drive-reskin:
https://github.com/fahimc/filebrowser/commit/5171894f714ed8ab8fa37890021fcd20eb5c2fc4
README local image targets verified in the staged Git tree before commit.
Original EXE release assets unchanged; no Actions build was used.
