// Local unsigned packaging; this script never uploads or publishes artifacts.
import { execFileSync } from 'node:child_process';
import { mkdir, copyFile, writeFile, readFile, mkdtemp, rm, symlink, chmod } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { APP_DISPLAY_NAME } from '../app-name.mjs';
const root=fileURLToPath(new URL('../../',import.meta.url));
const requested=process.argv[2];
const platform={win:'win32',mac:'darwin',linux:'linux'}[requested]||process.platform;
if(platform!==process.platform)throw new Error('Build the native installer on its target OS');
const requestedArch=process.argv[3]?.replace('x64','amd64');
if(requestedArch && requestedArch!==process.arch.replace('x64','amd64'))throw new Error('Build this installer on its target architecture');
const version=JSON.parse(await readFile(join(root,'package.json'),'utf8')).version;
const output=join(root,'release/native',`${process.platform}-${process.arch}`);
await mkdir(output,{recursive:true});
const run=(command,args,options={})=>execFileSync(command,args,{cwd:root,stdio:'inherit',...options});
run(process.execPath,['native/scripts/build.mjs']);
run(process.execPath,['native/scripts/check-package.mjs']);
const payload=join(root,'out/native/hwmusic',`${process.platform}-${process.arch}`);
let artifact;
if(platform==='darwin'){
 run(process.execPath,['native/scripts/build-macos.mjs']);
 const staging=await mkdtemp(join(tmpdir(),'hwmusic-native-dmg-'));
 try{
  run('ditto',[join(root,'out/native/hwmusic.app'),join(staging,`${APP_DISPLAY_NAME}.app`)]);
  run('codesign',['--verify','--deep','--strict',join(staging,`${APP_DISPLAY_NAME}.app`)]);
  await symlink('/Applications',join(staging,'Applications'));
  await copyFile(join(payload,'NOTICE.txt'),join(staging,'NOTICE.txt'));
  artifact=join(output,`hwmusic-${version}-${process.arch}-${process.env.HWMUSIC_MAC_SIGN_IDENTITY ? 'local-signed' : 'unsigned'}.dmg`);
  run('hdiutil',['create','-ov','-format','UDZO','-volname',APP_DISPLAY_NAME,'-srcfolder',staging,artifact]);
  run('hdiutil',['verify',artifact]);
 }finally{await rm(staging,{recursive:true,force:true});}
}else if(platform==='win32'){
 const staging=await mkdtemp(join(tmpdir(),'hwmusic-native-nsis-'));
 try{
  artifact=join(output,`hwmusic-${version}-${process.arch}-setup-unsigned.exe`);
  const esc=value=>value.replaceAll('$','$$').replaceAll('"','$\\"');
  const files=['hwmusic.exe','LICENSE','THIRD_PARTY_NOTICES.txt','icon.LICENSE.txt','NOTICE.txt','SHA256SUMS'];
  // LicenseData reads its own file encoding, independently of /INPUTCHARSET.
  const licenseFile=join(staging,'license-utf16.txt');
  await writeFile(licenseFile,Buffer.from('\ufeff'+APP_DISPLAY_NAME+' · 安装说明\r\n\r\n'+await readFile(join(payload,'NOTICE.txt'),'utf8'),'utf16le'));
  const legacyNames=[...new Set([
    Buffer.from(APP_DISPLAY_NAME,'utf8').toString('latin1'),
    new TextDecoder('windows-1252').decode(Buffer.from(APP_DISPLAY_NAME,'utf8')),
  ])].filter(name=>name!==APP_DISPLAY_NAME);
  const script=`Unicode true
!include "MUI2.nsh"
!define MUI_ICON "${esc(join(root,'out/native/windows-resources/app.ico'))}"
!define MUI_UNICON "${esc(join(root,'out/native/windows-resources/app.ico'))}"
Name "${esc(APP_DISPLAY_NAME)}"
OutFile "${esc(artifact)}"
RequestExecutionLevel user
InstallDir "$LOCALAPPDATA\\Programs\\hwmusic"
InstallDirRegKey HKCU "Software\\HuaweiMusicViva" "InstallDir"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${esc(licenseFile)}"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\\hwmusic.exe"
!define MUI_FINISHPAGE_RUN_TEXT "打开${esc(APP_DISPLAY_NAME)}"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
Section
 SetShellVarContext current
 SetOutPath "$INSTDIR"
${files.map(file=>` File "${esc(join(payload,file))}"`).join('\n')}
 WriteUninstaller "$INSTDIR\\Uninstall.exe"
 CreateDirectory "$SMPROGRAMS\\hwmusic"
 CreateDirectory "$INSTDIR\\upgrade-backup"
${legacyNames.map((name,i)=>` IfFileExists "$SMPROGRAMS\\hwmusic\\${esc(name)}.lnk" 0 +2
 Rename "$SMPROGRAMS\\hwmusic\\${esc(name)}.lnk" "$INSTDIR\\upgrade-backup\\legacy-shortcut-${i}.lnk.bak"`).join('\n')}
 CreateShortcut "$SMPROGRAMS\\hwmusic\\${esc(APP_DISPLAY_NAME)}.lnk" "$INSTDIR\\hwmusic.exe"
 CreateShortcut "$SMPROGRAMS\\hwmusic\\Uninstall.lnk" "$INSTDIR\\Uninstall.exe"
 WriteRegStr HKCU "Software\\HuaweiMusicViva" "InstallDir" "$INSTDIR"
 WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\HuaweiMusicViva" "DisplayName" "${esc(APP_DISPLAY_NAME)}"
 WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\HuaweiMusicViva" "DisplayVersion" "${version}"
 WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\HuaweiMusicViva" "UninstallString" '"$INSTDIR\\Uninstall.exe"'
SectionEnd
Section "Uninstall"
 SetShellVarContext current
${files.map(file=>` Delete "$INSTDIR\\${file}"`).join('\n')}
 Delete "$INSTDIR\\Uninstall.exe"
 Delete "$SMPROGRAMS\\hwmusic\\${esc(APP_DISPLAY_NAME)}.lnk"
 Delete "$SMPROGRAMS\\hwmusic\\Uninstall.lnk"
 RMDir "$SMPROGRAMS\\hwmusic"
 RMDir "$INSTDIR"
 DeleteRegKey HKCU "Software\\HuaweiMusicViva"
 DeleteRegKey HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\HuaweiMusicViva"
SectionEnd
`;
  const file=join(staging,'installer.nsi');await writeFile(file,'\ufeff'+script);
  run(process.env.HWMUSIC_MAKENSIS||'makensis',['/INPUTCHARSET','UTF8','/V2',file]);
 }finally{await rm(staging,{recursive:true,force:true});}
}else if(platform==='linux'){
 if (process.argv[4] === 'deb') {
  const staging=await mkdtemp(join(tmpdir(),'hwmusic-deb-'));
  try {
   await chmod(staging,0o755);
   for (const directory of ['DEBIAN', 'usr/bin', 'usr/share/applications', 'usr/share/icons/hicolor/256x256/apps', 'usr/share/doc/hwmusic']) await mkdir(join(staging,directory),{recursive:true});
   await copyFile(join(payload,'hwmusic'),join(staging,'usr/bin/hwmusic'));
   await copyFile(join(payload,'hwmusic.desktop'),join(staging,'usr/share/applications/hwmusic.desktop'));
   await copyFile(join(payload,'icon.png'),join(staging,'usr/share/icons/hicolor/256x256/apps/hwmusic.png'));
   for (const name of ['LICENSE','THIRD_PARTY_NOTICES.txt','icon.LICENSE.txt','NOTICE.txt','LINUX.md']) await copyFile(join(payload,name),join(staging,'usr/share/doc/hwmusic',name));
   await writeFile(join(staging,'DEBIAN/control'),`Package: hwmusic
Version: ${version}
Section: sound
Priority: optional
Architecture: ${{x64:'amd64',arm64:'arm64'}[process.arch] || process.arch}
Maintainer: hwmusic local build <noreply@example.invalid>
Depends: libc6 (>= 2.34), libxcb1, libvulkan1, libpango-1.0-0, libpangocairo-1.0-0, libcairo2, libfontconfig1, fontconfig
Recommends: mesa-vulkan-drivers, libpulse0, xwayland, fonts-noto-cjk
Description: hwmusic unofficial Huawei Music native desktop client
 Local unsigned Linux test build. Real account login requires a Chromium browser.
`);
   run('chmod',['-R','u=rwX,go=rX',staging]);
   artifact=join(output,`hwmusic-${version}-${process.arch}-unsigned.deb`);
   run('dpkg-deb',['--root-owner-group','--build',staging,artifact]);
  } finally { await rm(staging,{recursive:true,force:true}); }
 } else {
 artifact=join(output,`hwmusic-${version}-${process.arch}-unsigned.tar.gz`);
 run('tar',['-czf',artifact,'-C',payload,'hwmusic','LICENSE','THIRD_PARTY_NOTICES.txt','icon.LICENSE.txt','NOTICE.txt','SHA256SUMS','icon.png','install.sh','uninstall.sh','hwmusic.desktop','LINUX.md']);
 }
}else{throw new Error('Unsupported installer platform');}
const hash=createHash('sha256').update(await readFile(artifact)).digest('hex');
await writeFile(join(output,'SHA256SUMS'),`${hash}  ${artifact.split(/[\\/]/).at(-1)}\n`);
console.log(`Local test package: ${artifact}`);
