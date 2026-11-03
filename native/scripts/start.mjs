import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { existsSync } from 'node:fs';
import { join } from 'node:path';

const root = fileURLToPath(new URL('../../', import.meta.url));
let child;
function run(command, args, fallback = false) {
child = spawn(command, args, {
  cwd: fileURLToPath(new URL('../', import.meta.url)), stdio: 'inherit',
});
child.once('error', error => {
  const binary = join(root, 'out/native/hwmusic', `${process.platform}-${process.arch}`, process.platform === 'win32' ? 'hwmusic.exe' : 'hwmusic');
  if (!fallback && error.code === 'ENOENT' && !process.env.HWMUSIC_GO && existsSync(binary)) {
    console.log('未找到 Go，启动已构建的本地 Viva 程序；源码修改需重新构建。');
    run(binary, process.argv.slice(2), true);
    return;
  }
  console.error('需要 Go 1.26.10+ 与本平台 C 编译器；也可用 HWMUSIC_GO 指定 Go 路径。'); process.exitCode = 1;
});
child.once('exit', code => { process.exitCode = code ?? 1; });
}
const go = process.env.HWMUSIC_GO || 'go';
const probe = spawnSync(go, ['version'], { stdio: 'ignore' });
if (probe.status === 0) {
  const mac = process.platform === 'darwin';
  const build = spawnSync(process.execPath, [join(root, 'native/scripts', mac ? 'build-macos.mjs' : 'build.mjs')], { cwd: root, stdio: 'inherit' });
  if (build.status !== 0) process.exit(build.status ?? 1);
  // Run the built app from a stable location; credentials use the local profile.
  const binary = mac ? join(root, 'out/native/hwmusic.app/Contents/MacOS/hwmusic') : join(root, 'out/native/hwmusic', `${process.platform}-${process.arch}`, process.platform === 'win32' ? 'hwmusic.exe' : 'hwmusic');
  run(binary, process.argv.slice(2), true);
} else if (probe.error?.code === 'ENOENT' && !process.env.HWMUSIC_GO) {
  const bundle = join(root, 'out/native/hwmusic.app/Contents/MacOS/hwmusic');
  if (process.platform === 'darwin' && existsSync(bundle)) run(bundle, process.argv.slice(2), true);
  else run(go, ['version']);
} else {
  console.error('无法运行 Go，请检查 HWMUSIC_GO。');
  process.exitCode = 1;
}
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => child?.kill(signal));
