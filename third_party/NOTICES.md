# Third-party notices

## ranuts/document and ONLYOFFICE editors

FileBrowser bundles the browser editor built from [`ranuts/document`](https://github.com/ranuts/document), pinned at commit `1330b3c525ca4d7211f99592f5e94df749e5202a`. The upstream project is licensed under GNU AGPL version 3. Its distribution includes ONLYOFFICE editor components under AGPL-3.0 with additional terms described in the upstream notice.

The integration carries downstream source patches in [`patches/ranuts-document-disable-embedded-service-worker.patch`](../patches/ranuts-document-disable-embedded-service-worker.patch). It enables embedded new-document creation and readiness events without local document history, prevents the embedded iframe from registering a service worker against the FileBrowser origin, and maps Calibri's styled faces to the open Carlito faces. The patches are applied by the editor build scripts; the upstream source and its full `LICENSE` and `NOTICE` files are retained in the Git submodule.

The additional [collaboration export patch](../patches/ranuts-document-preserve-collaboration-history.patch) keeps unsent native editor changes intact during checkpoint and draft exports, retains the embedded converter worker for offline draft exports, and passes already loaded font bytes when disconnected.

When distributing a FileBrowser executable that contains this editor, distribute the corresponding source for the complete combined work under the applicable AGPL terms, including this repository, the pinned submodule source, and the downstream patch. A source checkout should be made with recursive submodules enabled:

```sh
git clone --recurse-submodules <repository-url>
```

See [`third_party/ranuts-document/LICENSE`](ranuts-document/LICENSE) and [`third_party/ranuts-document/NOTICE`](ranuts-document/NOTICE) for the complete upstream licensing terms and editor component notices.

The Office release also supplies engine source archives alongside the FileBrowser/ranuts source archive:

- [ONLYOFFICE sdkjs 9.3.0.133](https://github.com/ONLYOFFICE/sdkjs/tree/679a6d70ccb5487924f034cea089ff30daeeb7fd).
- [ONLYOFFICE web-apps 9.3.0.133](https://github.com/ONLYOFFICE/web-apps/tree/3d7f2370c2a1043d4e7839f5942f096c491923ef).
- [CryptPad x2t WebAssembly source and build scripts](https://github.com/cryptpad/onlyoffice-x2t-wasm/tree/742bf7e06fde1502c63dfe4aae1fe8c70272fc30), including its modified ONLYOFFICE core tree. The ranuts distribution supplies the compiled converter and identifies this upstream project in its NOTICE; it does not record a converter source commit or compiler pin, so this source snapshot is not claimed to reproduce that converter byte for byte.
