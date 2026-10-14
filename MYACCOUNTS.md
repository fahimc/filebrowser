# MyAccounts SSO

The companion source repository is [fahimc/myaccounts](https://github.com/fahimc/myaccounts). Its single native Windows EXE embeds the identity UI/service and SQLite. First execution asks for administrator credentials, domain/subdomain, optional Cloudflare method/token and SMTP details. Automatic Windows startup is optional and requests elevation only when chosen.

Use a separate OIDC client per FileBrowser origin, `openid profile email`, exact `ORIGIN/api/auth/oidc/callback`, and stable `sub` as userIdentifier. Keep password recovery administration available. New SSO users must have `userDefaults.account.permissions.admin: false` and no untested administrator-group mapping.

Configuration template:

```yaml
http:
  listen: 127.0.0.1
  trustProxyHeaders: true
frontend:
  oidcLoginButtonText: Sign in with MyAccounts
auth:
  methods:
    noauth: false
    password:
      enabled: true
      signup: false
    oidc:
      enabled: true
      issuerUrl: https://login.example.com
      clientId: YOUR_CLIENT_ID
      scopes: openid profile email
      userIdentifier: sub
      disableVerifyTLS: false
```

Supply `FILEBROWSER_OIDC_CLIENT_SECRET` through a protected environment. Proxy trust is appropriate only when the origin is reachable through the trusted proxy/loopback. The callback derives from the incoming browser-facing URL; externalUrl does not substitute for correct forwarded headers.

`scripts/run-myaccounts-filebrowser.ps1` reads a machine-DPAPI client handoff and supplies the secret only to the child environment. `scripts/configure-myaccounts.py` validates a selected FileBrowser configuration, backs up its SQLite settings using SQLite's backup API, and patches YAML without storing a new client secret. It preserves sources and Office history. These are optional configuration helpers; the packaged applications do not require Python/Node to run.

Current local and remote previews were connected to the authorized `https://login.m8e.co.uk` issuer using distinct clients and protected handoffs outside Git. Source files and version history remain in their original locations. The local preview changed from no-auth to SSO/password with a separate recovery administrator; the remote password administrator remains available. Do not distribute runtime files, credentials, cookies or temporary tunnel hostnames.

Verification: the v0.4.0 native executable passed actual Chrome redirects/callbacks for both local and public HTTPS previews, stable-sub linkage, normal-user permissions, other-administrator denial and HttpOnly/Secure public session cookies. MyAccounts email-first and consented saved-account select/change/remove also passed. Test: `scripts/myaccounts-sso-smoke.mjs`, requiring protected process credentials and a disposable browser profile. Existing Office/history release evidence remains in repository history; these changes configure authentication rather than altering the document engine.

The companion [verified development preview](https://github.com/fahimc/myaccounts/releases/tag/preview-20261014.1) passed Windows CI and is installed in protected NTFS storage on F:. Both previews passed browser SSO again against that exact released EXE. Full production/SMTP/service acceptance remains in the MyAccounts ledger.
