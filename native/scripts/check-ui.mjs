// Offline native UI checks. All profiles, tracks and screenshots are synthetic.
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdir, writeFile, mkdtemp, rm, readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

const root = fileURLToPath(new URL('../../', import.meta.url));
const native = join(root, 'native');
const go = process.env.HWMUSIC_GO || 'go';
const qa = join(root, 'out/native/qa');
await mkdir(qa, { recursive: true });
const binary = join(qa, process.platform === 'win32' ? 'music-headless.exe' : 'music-headless');
execFileSync(go, ['build', '-tags=vivahead', '-o', binary, './cmd/music'], { cwd: native, stdio: 'inherit' });
const profile = await mkdtemp(join(tmpdir(), 'hwmusic-viva-ui-'));
const report = { platform: process.platform, arch: process.arch, synthetic: true, privateAccount: false, layouts: [] };
try {
  for (const count of [2, 965]) {
    const commands = count === 2 ? [
      'layout', `shot ${join(qa,'library-current.png')}`, 'click "播放顺序"','layout',`shot ${join(qa,'mode-menu.png')}`,'click "随机播放"','click "标准"','layout',`shot ${join(qa,'quality-menu.png')}`,'click "无损音质"', 'click "调节音量"','layout',`shot ${join(qa,'volume-menu.png')}`,'click "调节音量"', 'click "设置"', 'layout',
      `shot ${join(qa,'settings-playback-current.png')}`, 'click "外观设置"', 'layout',
      `shot ${join(qa,'settings-appearance-current.png')}`,
      ...['晴海','鼠尾草','暖砂','蔷薇','冰川','午夜蓝','深林','石墨','夜玫瑰','咖啡','曜石','晨光'].flatMap(theme=>[`click "${theme}"`,'layout',`shot ${join(qa,`theme-${theme}.png`)}`,...(theme==='午夜蓝'?['click "调节音量"','layout',`shot ${join(qa,'volume-menu-dark.png')}`,'click "调节音量"']:[])]),
      'click "选择系统字体"','layout',`shot ${join(qa,'font-picker-current.png')}`,'click "关闭"',
      ...[1100,1020,960].flatMap(width => [`resize ${width} 640`, 'layout', `shot ${join(qa,`settings-appearance-${width}.png`)}`]),
      'click "播放设置"','scroll 800 420 0 550','layout',
      `shot ${join(qa,'settings-playback-scrolled-960.png')}`,
      'resize 1180 760','click "我的收藏"','click "专辑 Fixture Album"','layout',`shot ${join(qa,'album-current.png')}`,
      'click "返回"','click "艺人 Local fixture"','layout',`shot ${join(qa,'artist-current.png')}`,'click "返回"','click "我的歌单"','layout',`shot ${join(qa,'collections-current.png')}`,'click "离线测试歌单"','layout',`shot ${join(qa,'playlist-current.png')}`,
      'click "播放全部"','click "打开沉浸歌词"','settle 400','layout',`shot ${join(qa,'stage-current.png')}`,
      'resize 960 640','click "调节音量"','layout',`shot ${join(qa,'volume-menu-960.png')}`,'click "调节音量"','layout',`shot ${join(qa,'stage-960.png')}`,'click "歌词选项"','layout',`shot ${join(qa,'lyrics-menu-960.png')}`,'click "歌词提前半秒"','click "歌词选项"','click "退出沉浸歌词"','settle 400','click "播放队列"','layout',`shot ${join(qa,'drawer-current.png')}`,'quit',
    ] : ['click "播放全部"','waitfor "共 965 首收藏" 5000','click "播放队列"','layout',`shot ${join(qa,'queue-965.png')}`,'quit'];
    const run = spawnSync(binary, ['--demo', String(count), '--profile', profile], { cwd: root, env: { ...process.env, VIVA_HEADLESS: '1' }, input: commands.join('\n')+'\n', encoding: 'utf8', timeout: process.platform === 'linux' ? 120000 : 30000 });
    await writeFile(join(qa,`ui-${count}.txt`), (run.stdout||'')+(run.stderr||''));
    if (run.error || run.status !== 0 || /^error\b/m.test(run.stdout) || /ok [1-9]\d* problems/.test(run.stdout)) throw new Error(`Native UI check failed for ${count}; ${run.error?.message || `exit ${run.status}`}; see out/native/qa/ui-${count}.txt`);
    report.layouts.push({ tracks: count, checks: (run.stdout.match(/ok 0 problems/g)||[]).length });
  }
  const saved = JSON.parse(await readFile(join(profile, 'queue.json'), 'utf8'));
  if (saved.tracks.length !== 965) throw new Error('Native UI saved an incomplete queue');
  await writeFile(join(qa,'report.json'), JSON.stringify(report,null,2)+'\n');
  console.log(JSON.stringify(report,null,2));
} finally { await rm(profile,{recursive:true,force:true}); }
