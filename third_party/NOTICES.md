# Third-party notices

## ranuts/document and ONLYOFFICE editors

FileBrowser bundles the browser editor built from [`ranuts/document`](https://github.com/ranuts/document), pinned at commit `1330b3c525ca4d7211f99592f5e94df749e5202a`. The upstream project is licensed under GNU AGPL version 3. Its distribution includes ONLYOFFICE editor components under AGPL-3.0 with additional terms described in the upstream notice.

The integration carries one downstream source patch in [`patches/ranuts-document-disable-embedded-service-worker.patch`](../patches/ranuts-document-disable-embedded-service-worker.patch). It enables embedded new-document creation and readiness events without local document history, prevents the embedded iframe from registering a service worker against the FileBrowser origin, and maps Calibri's styled faces to the open Carlito faces. The source patch is applied by the editor build scripts; the upstream source and its full `LICENSE` and `NOTICE` files are retained in the Git submodule.

When distributing a FileBrowser executable that contains this editor, distribute the corresponding source for the complete combined work under the applicable AGPL terms, including this repository, the pinned submodule source, and the downstream patch. A source checkout should be made with recursive submodules enabled:

```sh
git clone --recurse-submodules <repository-url>
```

See [`third_party/ranuts-document/LICENSE`](ranuts-document/LICENSE) and [`third_party/ranuts-document/NOTICE`](ranuts-document/NOTICE) for the complete upstream licensing terms and editor component notices.
