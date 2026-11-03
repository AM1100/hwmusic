import { execFileSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { macSigningIdentity, signingConfig } from './sign-macos.mjs';

if (process.platform !== 'darwin') throw new Error('Local code signing setup is macOS only');
const name = 'hwmusic Local Development';
const run = (cmd, args) => execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout: 20000 });
const existing = macSigningIdentity();
if (existing) {
  console.log('A signing identity is already configured; keeping it unchanged.');
  process.exit(0);
}
const keychain = run('security', ['default-keychain', '-d', 'user']).trim().replace(/^"|"$/g, '');
let fingerprint;
try {
  const matches = run('security', ['find-certificate', '-c', name, '-Z', keychain]);
  fingerprint = matches.match(/SHA-1 hash: ([A-F0-9]{40})/)?.[1];
} catch (error) {
  if (error.status !== 44) throw error;
}
if (!fingerprint) {
  const scratch = mkdtempSync(join(tmpdir(), 'hwmusic-signing-'));
  try {
    const config = join(scratch, 'certificate.cnf');
    const cert = join(scratch, 'certificate.pem');
    const key = join(scratch, 'private.pem');
    const bundle = join(scratch, 'identity.p12');
    writeFileSync(config, `[req]
prompt = no
distinguished_name = subject
x509_extensions = signing
[subject]
CN = ${name}
[signing]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
subjectKeyIdentifier = hash
`, { mode: 0o600 });
    run('openssl', ['req', '-new', '-newkey', 'rsa:3072', '-nodes', '-x509', '-sha256', '-days', '3650', '-config', config, '-keyout', key, '-out', cert]);
    run('openssl', ['pkcs12', '-export', '-inkey', key, '-in', cert, '-name', name, '-out', bundle, '-passout', 'pass:hwmusic-local-import']);
    // Only codesign may use this private key; do not grant all-app access or
    // install a system-wide trusted root. Scratch is private and always removed.
    run('security', ['import', bundle, '-k', keychain, '-f', 'pkcs12', '-P', 'hwmusic-local-import', '-x', '-T', '/usr/bin/codesign']);
    fingerprint = run('openssl', ['x509', '-in', cert, '-noout', '-fingerprint', '-sha1']).split('=')[1].replaceAll(':', '').trim();
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}
if (!/^[A-F0-9]{40}$/.test(fingerprint)) throw new Error('Could not identify the local signing certificate');
mkdirSync(dirname(signingConfig), { recursive: true, mode: 0o700 });
writeFileSync(signingConfig, `${JSON.stringify({ fingerprint, name }, null, 2)}\n`, { mode: 0o600, flag: 'wx' });
console.log('Local signing identity configured. Private key stays in the user keychain; no global trust settings changed.');
