# Drive display, Studio cache and English Office fixes

Uncommitted local fixes: store verified OIDC displayName separately from immutable
username/subject in user metadata and render it in the sidebar. Fresh normal SSO
shows friendly display name. Studio iframe gets the build version and its HTML
is no-store; bundled Studio uses content-digest script URLs so upgrades replace
old cached editor code. Adapter changes are mirrored in the Studio repository.

New DOCX/PPTX use explicit English OOXML seed buffers under public/templates;
SDK vendor submodule unchanged. DOCX uses Arial/en-GB; PPTX layouts are English.
Browser created/saved both; disk ZIP confirmed text, English settings, two slides
and English layout labels. PPTX screenshot retained outside Git. Word screenshot
capture timed out, though DOM/save/file checks passed. Existing Office documents
preserved. Studio rotate/undo/redo/save verified 320x200 -> 200x320 without added
transparent corners. Frontend built via build:docker preserving Office payload.

Built/applied locally; no publication. Unrelated untracked preview files preserved.
Office complex documents/PDF/multi-user simultaneous editing remain untested.
