# <img src="frontend/src/assets/branding/my_drive.svg" alt="" width="40" height="40"> My Drive

A self-hosted file manager with a Drive-style interface, browser Office editing, photo browsing and an optional image editor. Built on FileBrowser Quantum and connected to [One Workspace](https://github.com/fahimc/one-workspace) through MyAccounts.

[Download Windows previews](https://github.com/fahimc/filebrowser/releases) · [MyAccounts setup](MYACCOUNTS.md) · [Office](OFFICE.md) · [Photos](PHOTOS.md) · [Suite installer](https://github.com/fahimc/one-workspace)

![My Drive browsing sample images, with Photos and Photo Editor in the sidebar](docs/images/drive.png)

*Actual interface capture from an isolated preview using sample files.*

## Your files, with more ways to work

| App | Workflow and guide |
| --- | --- |
| <img src="frontend/src/assets/branding/my_drive.svg" alt="" width="28" height="28"> **Drive** | Browse sources, upload, search and share. [FileBrowser documentation](https://filebrowserquantum.com/). |
| <img src="frontend/src/assets/branding/my_office.svg" alt="" width="28" height="28"> **Office** | Edit documents, sheets, slides and PDFs with autosave and history. [Office guide](OFFICE.md). |
| <img src="frontend/src/assets/branding/my_photos.svg" alt="" width="28" height="28"> **Photos** | Explore a thumbnail timeline, folder albums and favourites. [Photos guide](PHOTOS.md). |
| <img src="frontend/src/assets/branding/photo_editor.svg" alt="" width="28" height="28"> **Photo&nbsp;Editor** | Open images in Studio and save through Drive's permissions. [Studio app and adapter](https://github.com/fahimc/miniPaint/blob/master/STUDIO.md). |

All file access uses FileBrowser's source configuration and permissions. Office documents and photos stay in their existing folders; browsing them does not import them into a second storage service.

## Choose how to install

### With One Workspace

Use the [One Workspace installer](https://github.com/fahimc/one-workspace). Its wizard configures the included components and shared MyAccounts login. Apps open from the portal and can use the same HTTPS tunnel.

### Standalone Drive

Download a preview from this fork's [releases](https://github.com/fahimc/filebrowser/releases), configure your file sources and authentication, then launch the executable with its configuration:

```powershell
.\FileBrowser.exe -c .\config.yaml
```

FileBrowser can use its normal authentication independently of MyAccounts. To connect shared sign-in, follow [MYACCOUNTS.md](MYACCOUNTS.md). Keep the OIDC client secret in a protected launcher environment.

**Release coverage:** the published `drive-office-v0.4.0` package includes Office/version history. Newer Photos, Studio and branding source may require a local build or a later package; check each release's notes before installing.

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

Start with [contributing instructions](CONTRIBUTING.md). Additional build and integration guides:

- [Office editor assets and backend integration](OFFICE.md)
- [Portable Photos build](scripts/build-photos.ps1)
- [Photo Editor source and FileBrowser adapter](https://github.com/fahimc/miniPaint)
- [MyAccounts configuration](MYACCOUNTS.md)

No GitHub Actions build is required for the local packages. Runtime files, credentials and private previews belong outside the repository.

## Project background and licenses

This fork builds on [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser). The file manager retains its [Apache 2.0 license](LICENSE); bundled editor components retain their own licenses, including AGPL terms. Read [third-party notices](third_party/NOTICES.md) when building or redistributing.

The [earlier README](docs/README-archive.md) preserves upstream background, credits and historical comparisons. Use the guides above for this fork's current setup.
