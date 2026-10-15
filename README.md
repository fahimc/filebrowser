<p align="center">
  <img src="frontend/src/assets/branding/my_drive.svg" alt="My Drive logo" width="112" height="112">
</p>

<h1 align="center">My Drive</h1>

A self-hosted file manager with a Drive-style interface, browser Office editing, photo browsing and an optional image editor. Built on FileBrowser Quantum and connected to [One Workspace](https://github.com/oneworkspacestudio/one-workspace) through MyAccounts.

[Download Windows previews](https://github.com/oneworkspacestudio/drive/releases) · [MyAccounts setup](MYACCOUNTS.md) · [Office](OFFICE.md) · [Photos](PHOTOS.md) · [Suite installer](https://github.com/oneworkspacestudio/one-workspace)

![Current My Drive interface with shared controls and compact sidebar selection](docs/images/drive-current.png)

*Earlier interface preview with sample files. The current project logos are shown above and in the app table; screenshots may retain older branding. Check the release notes for the included app versions.*

## Your files, with more ways to work

| App | Workflow and guide |
| --- | --- |
| <img src="frontend/src/assets/branding/my_drive.svg" alt="" width="28" height="28"> **Drive** | Browse sources, upload, search and share. [FileBrowser documentation](https://filebrowserquantum.com/). |
| <img src="frontend/src/assets/branding/my_office.svg" alt="" width="28" height="28"> **Office** | Edit documents, sheets, slides and PDFs with autosave and history. [Office guide](OFFICE.md). |
| <img src="frontend/src/assets/branding/my_photos.svg" alt="" width="28" height="28"> **Photos** | Explore a thumbnail timeline, folder albums and favourites. [Photos guide](PHOTOS.md). |
| <img src="frontend/src/assets/branding/photo_editor.svg" alt="" width="28" height="28"> **Photo&nbsp;Editor** | Open images in Studio and save through Drive's permissions. [Studio app and adapter](https://github.com/oneworkspacestudio/Photo-Editor/blob/master/STUDIO.md). |

All file access uses FileBrowser's source configuration and permissions. Office documents and photos stay in their existing folders; browsing them does not import them into a second storage service.

## Choose how to install

### With One Workspace

Use the [One Workspace installer](https://github.com/oneworkspacestudio/one-workspace). Its wizard configures the included components and shared MyAccounts login. Apps open from the portal and can use the same HTTPS tunnel.

### Standalone Drive

Download a preview from this fork's [releases](https://github.com/oneworkspacestudio/drive/releases), configure your file sources and authentication, then launch the executable with its configuration:

```powershell
.\FileBrowser.exe -c .\config.yaml
```

FileBrowser can use its normal authentication independently of MyAccounts. To connect shared sign-in, follow [MYACCOUNTS.md](MYACCOUNTS.md). Keep the OIDC client secret in a protected launcher environment.

**Release coverage:** the [v0.5.0 preview](https://github.com/oneworkspacestudio/drive/releases) packages the shared Drive design, Office/version history, Photos and Image Studio, with improved upload confirmation and recovery. Check the release notes for package contents and current limits.

## Office and Photos previews

<details>
<summary>Office document editing</summary>

![Office document editing with the My Office toolbar logo](docs/images/office.png)

Open a supported file, edit in your browser and wait for **All changes saved**. See [Office setup, history and build details](OFFICE.md).

</details>

<details>
<summary>Photos library</summary>

![My Photos library showing sample images](docs/images/photos.png)

The timeline uses file modification dates. Albums correspond to ordinary folders, and favourites use the existing per-user pinned items. Android uploads are deferred; see [PHOTOS.md](PHOTOS.md).

</details>

## Access and storage

- Sources can point at existing folders. Configure scopes, access rules and read/write permissions for each user or group.
- Integrated Office, Photos and Studio use the signed-in Drive user's access. Standalone companions have their own connection requirements.
- Shared suite folders are shared storage; they do not create private home directories automatically.
- Back up your files, FileBrowser settings/database and document history. Stop writes before taking filesystem snapshots.

## Build and integration

The custom app is maintained on [`master`](https://github.com/oneworkspacestudio/drive/tree/master), the default branch of this fork.

Start with [contributing instructions](CONTRIBUTING.md). Additional build and integration guides:

- [Office editor assets and backend integration](OFFICE.md)
- [Portable Photos build](scripts/build-photos.ps1)
- [Photo Editor source and FileBrowser adapter](https://github.com/oneworkspacestudio/Photo-Editor)
- [MyAccounts configuration](MYACCOUNTS.md)
- [Shared Drive design system and component reference](docs/DESIGN-SYSTEM.md)

No GitHub Actions build is required for the local packages. Runtime files, credentials and private previews belong outside the repository.

## Project background and licenses

This fork builds on [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser). The file manager retains its [Apache 2.0 license](LICENSE); bundled editor components retain their own licenses, including AGPL terms. Read [third-party notices](third_party/NOTICES.md) when building or redistributing.

The [earlier README](docs/README-archive.md) preserves upstream background, credits and historical comparisons. Use the guides above for this fork's current setup.
