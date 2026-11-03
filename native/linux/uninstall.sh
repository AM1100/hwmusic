#!/bin/sh
set -eu
app_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
data_dir=$(dirname -- "$app_dir")
if [ "$(basename -- "$app_dir")" != hwmusic-app ]; then
    echo '请运行安装目录内的 uninstall.sh' >&2
    exit 1
fi
rm -f -- "$data_dir/applications/hwmusic.desktop" "$data_dir/icons/hicolor/256x256/apps/hwmusic.png"
for name in hwmusic LICENSE THIRD_PARTY_NOTICES.txt icon.LICENSE.txt NOTICE.txt SHA256SUMS icon.png uninstall.sh LINUX.md; do
    rm -f -- "$app_dir/$name"
done
rmdir -- "$app_dir" 2>/dev/null || :
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$data_dir/applications" || :; fi
echo 'hwmusic已卸载；用户配置与登录数据保留。'
