import { execFileSync } from 'node:child_process';
import { mkdir, writeFile, readFile, copyFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { APP_DISPLAY_NAME } from '../app-name.mjs';
import { writeNotices } from './licenses.mjs';
import { signMacOS } from './sign-macos.mjs';

if (process.platform !== 'darwin') throw new Error('This build requires macOS');
const root = fileURLToPath(new URL('../../', import.meta.url));
const version = JSON.parse(await readFile(join(root, 'package.json'), 'utf8')).version;
const contents = join(root, 'out/native/hwmusic.app/Contents');
await mkdir(join(contents, 'MacOS'), { recursive: true });
await mkdir(join(contents, 'Resources'), { recursive: true });
// A prior certificate-signed build must not leave a stale bundle seal when
// rebuilding without an explicit signing identity.
await rm(join(contents, '_CodeSignature'), { recursive: true, force: true });
await copyFile(join(root,'build/icon.png'),join(contents,'Resources/icon.png'));
const scratch = await mkdtemp(join(tmpdir(), 'hwmusic-icon-'));
try {
  const iconset = join(scratch, 'AppIcon.iconset');
  await mkdir(iconset);
  for (const size of [16, 32, 128, 256, 512]) {
    for (const scale of [1, 2]) {
      const output = join(iconset, `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`);
      execFileSync('sips', ['-z', String(size*scale), String(size*scale), join(root,'build/icon-1024.png'), '--out', output], { stdio: 'ignore' });
    }
  }
  execFileSync('iconutil', ['-c', 'icns', iconset, '-o', join(contents,'Resources/AppIcon.icns')]);
} finally { await rm(scratch, { recursive: true, force: true }); }
await copyFile(join(root,'build/icon.LICENSE.txt'),join(contents,'Resources/icon.LICENSE.txt'));
await copyFile(join(root,'LICENSE'),join(contents,'Resources/LICENSE'));
await writeNotices(join(contents,'Resources/THIRD_PARTY_NOTICES.txt'));
execFileSync(process.env.HWMUSIC_GO || 'go', [
  'build', '-trimpath', '-ldflags=-s -w', '-o', join(contents, 'MacOS/hwmusic'), './cmd/music',
], { cwd: join(root, 'native'), stdio: 'inherit' });
const xml = value => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
// No CFBundleURLTypes: AuthenticationServices receives hms only for the attempt.
await writeFile(join(contents, 'Info.plist'), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>org.hwmusicdesktop.unofficial.internal.viva</string>
<key>CFBundleName</key><string>${xml(APP_DISPLAY_NAME)}</string>
<key>CFBundleDisplayName</key><string>${xml(APP_DISPLAY_NAME)}</string>
<key>CFBundleExecutable</key><string>hwmusic</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIconFile</key><string>AppIcon.icns</string>
<key>LSApplicationCategoryType</key><string>public.app-category.music</string>
<key>CFBundleVersion</key><string>${xml(version)}</string>
<key>CFBundleShortVersionString</key><string>${xml(version)}</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`);
execFileSync('plutil', ['-lint', join(contents, 'Info.plist')], { stdio: 'inherit' });
signMacOS(join(contents, '..'));
console.log('Built local Viva app (not notarized; no global hms handler).');
