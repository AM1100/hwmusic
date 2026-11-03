import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { signMacOS, macSigningIdentity } from './sign-macos.mjs';

if (process.platform !== 'darwin' || !macSigningIdentity()) throw new Error('A configured macOS signing identity is required');
process.env.HWMUSIC_MAC_SIGN_IDENTITY ||= macSigningIdentity();
const root = fileURLToPath(new URL('../', import.meta.url));
const scratch = mkdtempSync(join(tmpdir(), 'hwmusic-signing-check-'));
const binary = join(scratch, 'probe');
const source = join(scratch, 'main.go');
const service = `org.hwmusicdesktop.signing-test.${randomUUID()}`;
const run = (command, args) => execFileSync(command, args, { cwd: root, encoding: 'utf8', timeout: 30000, stdio: ['ignore', 'pipe', 'pipe'] });
let stored = false;
try {
  writeFileSync(source, `package main
import ("bytes"; "fmt"; "os"; "github.com/vivaui/viva/app")
var version string
func main() {
 service, op := os.Args[1], os.Args[2]
 value := []byte("synthetic-signing-check-not-a-login")
 var err error
 switch op {
 case "store": err = app.StoreSecret(service, "synthetic", value)
 case "load":
  var got []byte
  got, err = app.LoadSecret(service, "synthetic")
  if err == nil && !bytes.Equal(got,value) { err = fmt.Errorf("synthetic value mismatch") }
 case "delete": err = app.DeleteSecret(service,"synthetic")
 }
 if err != nil { fmt.Fprintln(os.Stderr, "synthetic vault operation failed:",err); os.Exit(1) }
 fmt.Println(version,op,"ok")
}
`, { mode: 0o600 });
  const signatures = [];
  for (const version of ['before-rebuild', 'after-rebuild']) {
    run(process.env.HWMUSIC_GO || 'go', ['build', '-ldflags', `-X main.version=${version}`, '-o', binary, source]);
    signMacOS(binary);
    // codesign prints its designated requirement to stdout and diagnostics to stderr.
    signatures.push(run('codesign', ['-d', '-r-', binary]).trim());
    const operation = stored ? 'load' : 'store';
    console.log(run(binary, [service, operation]).trim());
    stored = true;
  }
  assert.ok(signatures[0].includes('designated =>'), 'Missing designated requirement');
  assert.ok(!signatures[0].includes('cdhash'), 'Identity still depends on executable hash');
  assert.equal(signatures[0], signatures[1], 'Rebuilding changed the application identity');
  console.log('Stable identity and synthetic keychain access across rebuilds verified.');
} finally {
  try {
    if (stored) console.log(run(binary, [service, 'delete']).trim());
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}
