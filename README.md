# hwmusic

华为音乐终端播放器。在终端浏览收藏与歌单、搜索歌曲、播放音乐和查看歌词，日常使用无需打开 GUI。

基于 [听澜 Tinglan](https://github.com/talongao/tinglan) 的华为音乐协议与音频模块开发，键盘交互参考 [go-musicfox](https://github.com/go-musicfox/go-musicfox)。保留原作者版权，代码采用 **AGPL-3.0-only**。

## 构建与安装

需要 Linux、Go 1.26.10+、C 编译器、Node.js 22.15+，以及可用的 PulseAudio/PipeWire 或 ALSA 音频设备。

```sh
npm run install:tui
hwmusic
```

构建脚本支持 `HWMUSIC_GO=/绝对路径/go`。安装到 `~/.local/bin/hwmusic`，请确保此目录在 PATH 中。
编译后运行不需要 Go、Node.js、Viva、XWayland 或 Vulkan。

首次启动按 **大写 L**，在独立浏览器窗口用自己的华为账号扫码。授权完成后自动返回终端；后续启动校验并恢复保存的音乐会话。登录需要 Chrome/Chromium/Edge，日常播放无需启动浏览器。会话失效后重新登录，不自动续期。

## 常用按键

| 按键 | 操作 |
| --- | --- |
| `1` / `2` / `3` / `4` | 收藏 / 歌单 / 历史 / 队列 |
| `↑` / `↓` 或 `k` / `j` | 选择歌曲 |
| `Enter` / `Space` | 播放选中歌曲 / 暂停或继续 |
| `P` / `a` | 播放完整收藏或歌单 / 加入队尾 |
| `/` | 搜索，支持中文输入 |
| `n` / `p` | 下一首 / 上一首 |
| `←` / `→` | 快退 / 快进 5 秒 |
| `+` / `-` | 音量 |
| `v` / `?` | 歌词 / 完整按键帮助 |
| `L` / `U` | 登录 / 退出账号 |
| `q` | 保存队列和进度并退出 |

完整说明、配置路径、音质、循环模式及离线演示见 [终端版说明](native/TERMINAL.md)。原生桌面源码仍保留，参见 [桌面版说明](docs/DESKTOP.md)。

## 验证范围

在 Ubuntu 26.04.1 x86_64 上通过全模块 Go 测试、vet、终端状态 race、无图形显示环境的伪终端键盘检查，以及合成 MP3/FLAC、浏览器回调和隔离 MPRIS 测试。
真实华为账号扫码、账号会话恢复和在线音乐仍待当前用户验收，合成测试不代表真实账号已通过验证。

## 项目边界

非官方、实验性项目，与华为无隶属或合作关系。仅播放账号权益允许的华为音乐音源，不绕过会员、地域或 DRM 限制，不提供音频下载、解密或其他平台音源。登录使用的 OAuth 身份不属于本项目。

配置兼容目录保持为 `~/.config/huawei-music-desktop/terminal-v1`，避免更名导致登录、队列和设置丢失。凭据保护依赖操作系统用户隔离与文件权限，不读取 musicfox 或桌面版的私人会话。

请参阅 [免责声明](docs/DISCLAIMER.md)、[LICENSE](LICENSE)、[NOTICE](NOTICE) 与 [实现状态](docs/IMPLEMENTATION_STATUS.md)。原作者署名、上游历史记录和图标许可保留。
