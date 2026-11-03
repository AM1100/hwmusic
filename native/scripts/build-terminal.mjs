// Build/install the terminal frontend; no desktop window is started.
import { execFileSync } from 'node:child_process';
import { mkdir, copyFile, writeFile, readFile, chmod, rename, lstat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { createHash } from 'node:crypto';
import { writeNotices } from './licenses.mjs';

const root=fileURLToPath(new URL('../../',import.meta.url));
const output=join(root,'out/hwmusic',`${process.platform}-${process.arch}`);
const binary=process.platform==='win32'?'hwmusic.exe':'hwmusic';
await mkdir(output,{recursive:true});
execFileSync(process.env.HWMUSIC_GO||'go',['build','-trimpath','-ldflags=-s -w','-o',join(output,binary),'./cmd/hwmusic'],{cwd:join(root,'native'),stdio:'inherit'});
await copyFile(join(root,'LICENSE'),join(output,'LICENSE'));
await copyFile(join(root,'native/TERMINAL.md'),join(output,'README.md'));
await writeNotices(join(output,'THIRD_PARTY_NOTICES.txt'));
await writeFile(join(output,'NOTICE.txt'),'hwmusic · 非官方华为音乐终端播放器。本地未签名测试构建。\nOAuth 身份不属于本项目；真实账号授权与在线音乐仍需用户验收。\n仅使用账号权益允许的华为音乐音源，不绕过会员、地域或 DRM 限制。\n');
const hash=createHash('sha256').update(await readFile(join(output,binary))).digest('hex');
await writeFile(join(output,'SHA256SUMS'),`${hash}  ${binary}\n`);
if(process.argv.includes('--install')) {
 if(process.platform!=='linux') throw new Error('Automatic user installation currently supports Linux only');
 const bin=join(homedir(),'.local/bin');
 const data=join(process.env.XDG_DATA_HOME||join(homedir(),'.local/share'),'hwmusic');
 await mkdir(bin,{recursive:true});await mkdir(data,{recursive:true});
 const target=join(bin,binary);
 const existing=await lstat(target).catch(error=>{if(error.code==='ENOENT')return null;throw error;});
 if(existing && !existing.isFile()) throw new Error(`Refusing to replace a non-regular file: ${target}`);
 const temp=join(bin,`.hwmusic-${process.pid}.tmp`);
 await copyFile(join(output,binary),temp);await chmod(temp,0o755);await rename(temp,target);
 for(const name of ['LICENSE','README.md','THIRD_PARTY_NOTICES.txt','NOTICE.txt','SHA256SUMS']) await copyFile(join(output,name),join(data,name));
 console.log(`Installed ${target}\nRun: hwmusic`);
} else console.log(`Built terminal player: ${output}`);
