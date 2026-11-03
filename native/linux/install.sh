#!/bin/sh
# Per-user installation; no root access and no OAuth protocol registration.
set -eu
source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
(cd "$source_dir" && sha256sum -c SHA256SUMS)
data_dir=${XDG_DATA_HOME:-"$HOME/.local/share"}
case "$data_dir" in /*) ;; *) echo 'XDG_DATA_HOME must be an absolute path' >&2; exit 1 ;; esac
app_dir="$data_dir/hwmusic-app"
mkdir -p "$app_dir" "$data_dir/applications" "$data_dir/icons/hicolor/256x256/apps"
for name in hwmusic LICENSE THIRD_PARTY_NOTICES.txt icon.LICENSE.txt NOTICE.txt SHA256SUMS icon.png uninstall.sh LINUX.md; do
    install -m 644 "$source_dir/$name" "$app_dir/$name"
done
chmod 755 "$app_dir/hwmusic" "$app_dir/uninstall.sh"
install -m 644 "$source_dir/icon.png" "$data_dir/icons/hicolor/256x256/apps/hwmusic.png"
# Desktop Exec has its own escaping rules, independently of shell quoting.
exec_path=$(printf '%s' "$app_dir/hwmusic" | sed 's/[\\"`$]/\\\\&/g; s/%/%%/g')
sed "s|^Exec=.*|Exec=\"$(printf '%s' "$exec_path" | sed 's/[\\&|]/\\&/g')\"|" "$source_dir/hwmusic.desktop" > "$data_dir/applications/hwmusic.desktop"
chmod 644 "$data_dir/applications/hwmusic.desktop"
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$data_dir/applications" || :; fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then gtk-update-icon-cache -f -t "$data_dir/icons/hicolor" >/dev/null 2>&1 || :; fi
printf '已安装hwmusic。可从应用菜单打开，或运行：\n%s\n' "$app_dir/hwmusic"
