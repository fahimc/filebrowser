"""Patch a known FileBrowser configuration after an authorized installer handoff.

Client secrets remain in the protected handoff and are supplied by the Windows launcher.
This helper backs up settings with SQLite's backup API before changing YAML.
"""
import argparse, json, os, pathlib, sqlite3, urllib.parse
import yaml

p = argparse.ArgumentParser()
p.add_argument('--config', required=True)
p.add_argument('--client', required=True)
p.add_argument('--backup', required=True)
p.add_argument('--port', required=True, type=int)
p.add_argument('--recovery-user')
a = p.parse_args()
config = pathlib.Path(a.config).resolve()
backup = pathlib.Path(a.backup).resolve()
client = json.loads(pathlib.Path(a.client).read_text(encoding='utf-8-sig'))
original = config.read_bytes()
data = yaml.safe_load(original)
if not isinstance(data, dict) or data.get('http', {}).get('port') != a.port:
    raise SystemExit('Configuration is not the expected FileBrowser instance')
db_name = data.get('server', {}).get('database', {}).get('path')
if not db_name or not data.get('server', {}).get('sources'):
    raise SystemExit('Configuration does not identify a FileBrowser settings database and sources')
db = pathlib.Path(db_name)
if not db.is_absolute(): db = config.parent / db
if not db.is_file(): raise SystemExit('Settings database does not exist')
backup.mkdir(parents=True, exist_ok=False)
(backup / 'config.yaml').write_bytes(original)
uri = 'file:' + urllib.parse.quote(db.as_posix(), safe='/:') + '?mode=ro'
with sqlite3.connect(uri, uri=True) as source, sqlite3.connect(str(backup / 'settings.sqlite')) as dest:
    source.backup(dest)
    if dest.execute('PRAGMA quick_check').fetchone()[0] != 'ok': raise SystemExit('Backup integrity failed')
if data['http'].get('listen') not in ('127.0.0.1', 'localhost', '::1'):
    raise SystemExit('Proxy-header trust requires a loopback-bound origin')
data['http']['trustProxyHeaders'] = True
methods = data.setdefault('auth', {}).setdefault('methods', {})
if methods.get('oidc', {}).get('enabled'): raise SystemExit('Existing OIDC configuration will not be replaced automatically')
methods['noauth'] = False
password = methods.setdefault('password', {})
password['enabled'] = True
password['signup'] = False
if a.recovery_user: password['adminUsername'] = a.recovery_user
methods['oidc'] = dict(enabled=True, clientId=client['clientId'], issuerUrl=client['issuer'],
    scopes='openid profile email', userIdentifier='sub', disableVerifyTLS=False,
    adminGroup='', userGroups=[], groupsClaim='groups')
data.setdefault('frontend', {})['oidcLoginButtonText'] = 'Sign in with MyAccounts'
permissions = data.setdefault('userDefaults', {}).setdefault('account', {}).setdefault('permissions', {})
permissions['admin'] = False
permissions['api'] = False
temporary = config.with_name(config.name + '.myaccounts-new')
with open(temporary, 'x', encoding='utf-8') as out:
    yaml.safe_dump(data, out, allow_unicode=True, sort_keys=False)
    out.flush(); os.fsync(out.fileno())
os.replace(temporary, config)
print('FileBrowser configuration linked; SQLite/config backups retained; SSO defaults non-admin.')
