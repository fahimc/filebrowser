# Shared Drive design system

User supplied FileBrowser/Google Drive screenshots and requested consistent
buttons and elements across FileBrowser. Work is on local master, following the
earlier promotion from drive-reskin. This request authorizes implementation;
no commit, push, executable rebuild/release or live-instance update was performed.
Keep GitHub Actions disabled.

## Implementation

- New `frontend/src/css/drive-tokens.css` owns semantic light/dark colours,
  spacing, radii, control sizes, shadows and reduced-motion duration. Existing
  colour variables alias these roles, preserving user accent preferences.
- `drive-controls.css` supplies reusable button variants, icon controls, input
  and focus states, menu/dialog surfaces and legacy component adapters.
  `drive.css` now consumes tokens instead of maintaining a second palette.
- Removed global hover scaling; fixed selected-card height jumps, dialog footer
  clipping and dropdown hover shrink. New appears first in actual sidebar DOM;
  Photos/Studio remain with navigation instead of being pushed to the bottom.
- PWA/update banners use quiet surfaces and native close buttons. Sidebar
  quick toggles are native buttons with names/pressed state. Listing shortcuts
  now respect native interactive controls/default-prevented events so Enter
  activates buttons rather than being consumed as file navigation.
- Shared settings dropdown/switch styles consume the same tokens. The mobile
  drawer is bounded/scrollable and never shrinks the file area.
- `docs/DESIGN-SYSTEM.md` documents usage/provenance; development-only
  `frontend/design-system.html` demonstrates controls and light/dark selection.
  README links the guide. No Google application CSS/assets are vendored:
  measured visible New/view controls and adapted original local CSS to app markup,
  using public Material colour/button references. Embedded editor interiors
  retain their own styles.

## Verification and boundaries

Frontend production build and targeted CSS lint pass. ESLint passes for changed
Sidebar, ListingView, ExpandDropdown and ToggleSwitch files. General.vue has two
pre-existing raw-text i18n errors (Photos/Image Studio); the same errors were
confirmed with HEAD piped to ESLint. No new ESLint errors in that component.

Real headless Edge checks on isolated loopback19974 passed folder creation,
New menu/dialog, light/dark mode, unchanged hover dimensions, stable selection
dimensions in all five listing modes, 320/430/768px no horizontal overflow, mobile
drawer/PWA banner and Enter dismissal, settings/dropdown and component gallery.
No browser page errors. PWA availability is simulated for banner testing; no
actual OS installation is claimed. Fixture uses a separate NTFS database/files
at H:/Projects/drive-design-review with a prebuilt backend serving current built
frontend assets. Existing local/public installations were not touched.

Final reference captures are under docs/images/design-system (components in
both themes, desktop app and mobile drawer). Extra harness attempts initially
triggered the existing 500ms repeated-click navigation or measured a dropdown
during viewport transition; fresh navigation/viewport-before-load checks confirmed
stable dimensions. Task-owned preview stopped after verification. Temp scripts,
logs and runtime remain outside Git under H:/Projects/drive-design-review.

## Source-highlight correction

User subsequently rejected the remaining large My Drive capsule and storage
badge. Navigation now uses 8px corners and softer dedicated nav-selection tokens.
Source selection colours only the 40px name row; a 4px meter and separate muted
text sit beneath it. Links.vue configures the existing ProgressBar size/colours;
sidebar-scoped CSS reflows its label without changing global progress behaviour.
Reduced the status dot to 8px diameter. Other source categories/hybrid values and
their status colours remain driven by the existing data. The reference page and
screenshots must show this correction, replacing the earlier capsule captures.

Correction verified in light/dark and phone renders: source outer background is
transparent, selected name row is 40px high with 8px corners, meter is 4px high,
usage text flows below it. Targeted Links.vue ESLint, CSS lint and frontend build
pass. Updated all tracked design captures and added source-navigation.png.

Preserve concurrent unrelated Photos/API/upload/build-script edits and
`httpError.js`, plus original untracked drive-preview.png/frontend preview HTML.
Stage only this task's design-system files if later publication is requested.

## Publication scope

The user now explicitly requested pushing these Drive changes to master.
Publish this design system, navigation correction, component reference,
screenshots, README link and handoff together to fork/master. Remote master
matched the local base (47e23a95) before publication; GitHub default is master
and Actions remains disabled. Fresh targeted CSS lint passes. Existing build
and browser verification above applies; no executable release is requested.
Concurrent Photos/upload files remain outside this commit, as do the two
original untracked preview artifacts. Use a normal push to the user-owned
fahimc/filebrowser fork, never the upstream origin remote.
