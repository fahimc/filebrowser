# README presentation and product showcase

User requested a generated product showcase replacing the Workspace logo sheet,
and a formatting/content review across the five suite repositories. Existing
source publication authorization applies to these documentation updates. Keep
GitHub Actions disabled; no EXE release or application change is part of this task.

Main README now introduces this Drive fork, its actual app workflow and this fork's download/setup links. Office, Photos and Studio logos sit beside their workflow descriptions. Added docs/images/{drive,office,photos}.png actual UI captures; Photos/Drive and Studio documentation captures use isolated sample-image fixtures. Existing long upstream README/comparison content preserved in docs/README-archive.md. OFFICE.md/PHOTOS.md headings use compact inline logos. MYACCOUNTS.md clarifies that email/SMTP setup is optional.

Verification: local GFM previews visually inspected for all five main READMEs at
desktop and430px widths. Main/guide H1 counts, code-fence balance, Markdown whitespace
and local image/link targets checked. Archived local links rebased and verified.
No application test suites rerun for this documentation-only change.

Documentation screenshots used task-owned loopback19971/19972 processes and a fresh
H:/Projects/one-workspace-readme-review fixture with system sample images and a
synthetic document. Those processes were stopped. No live applications, accounts,
files, DNS or published binaries changed. Older previews/screenshots remain intact.

Publish only the listed README/guides/archive/image/history paths. Preserve other
ongoing work. The generation source and local render scratch remain outside Git;
all consumed final assets are repository-local. Upstream attribution/licenses are
retained and linked from each main README.

Publication verified on fork/drive-reskin at 8fa6ed6ebd01c4a61ab95c03f0140cb51ace5d79.
The fork default branch remains upstream-tracking main. The current README
explicitly points viewers/builders at drive-reskin; do not switch the default
branch or rewrite upstream main as part of a documentation-only continuation.

## Branch promotion requested after publication

The user subsequently requested merging drive-reskin into master. This supersedes
the earlier default-branch restriction above. No master existed on the fork;
created it from fork/main (91bc608f) and fast-forwarded drive-reskin (620fe412).
All custom app and README commits are retained, with no source-tree merge edits.
README now identifies master as the canonical branch. Published master to the
user-owned fork and verified it is GitHub's default. Main and drive-reskin remain
intact. Verified drive-reskin is an ancestor of master, with only README/history
differences. GitHub Actions remains disabled. The two untracked preview artifacts
are preserved. No executable rebuild or runtime tests were performed for this
branch promotion. Workspace has unrelated active edits and was left untouched.
