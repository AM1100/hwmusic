# Linux 本地测试版

解压后运行 `sh install.sh`，随后从应用菜单打开“hwmusic”。安装到
`${XDG_DATA_HOME:-$HOME/.local/share}/hwmusic-app`，无需管理员权限，运行不需要 Go 或 Node.js。
也可直接运行解压目录里的 `./hwmusic`。

卸载：运行安装目录内的 `sh uninstall.sh`，保留用户配置。
配置与会话仍使用 `${XDG_CONFIG_HOME:-$HOME/.config}/hwmusic/native-v1`。

依赖：X11/XWayland、Vulkan loader 与可用显卡驱动（或 Mesa lavapipe）、
XCB、Pango/Cairo、Fontconfig、D-Bus 会话和 PulseAudio/PipeWire 音频兼容服务。
登录需要 Google Chrome、Chromium 或 Microsoft Edge；可用
`HWMUSIC_AUTH_BROWSER=/绝对路径/浏览器 ./hwmusic` 指定浏览器。
使用独立临时浏览器配置自动接收回调，不注册全局 hms 协议。

Ubuntu/Debian 常用运行依赖（由系统管理员安装）：

```sh
sudo apt install libxcb1 libvulkan1 mesa-vulkan-drivers libpango-1.0-0 libpangocairo-1.0-0 libcairo2 libfontconfig1 fontconfig libpulse0 libasound2t64 xwayland fonts-noto-cjk
```

Wayland 桌面通过 XWayland 显示。托盘是否可见取决于桌面是否提供
StatusNotifier/AppIndicator 支持。真实扫码、账号权益和音乐播放需使用自己的账号验收。
这是非官方、未签名测试版；OAuth 身份不属于本项目，不绕过会员或版权限制。
在较新 Ubuntu 构建的二进制不保证可用于旧版 glibc；兼容旧系统应在目标系统重新构建。
