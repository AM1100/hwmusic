package main

import (
	"flag"
	"fmt"
	"os"

	"hwmusic/native/internal/terminal"
)

func main() {
	demo := flag.Bool("demo", false, "离线合成演示，不读取账号或请求业务网络")
	fixture := flag.String("fixture-dir", "", "演示用合成 tone.mp3 / tone.flac 目录（默认静音）")
	profile := flag.String("profile", "", "终端专用配置目录（默认 ~/.config/huawei-music-desktop/terminal-v1）")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "hwmusic · 华为音乐终端播放器\n\n用法：hwmusic [选项]\n\n首次启动按 L 在浏览器扫码；之后可直接在终端听歌。\n按 ? 查看所有快捷键，q 退出。\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if err := terminal.Run(*profile, *fixture, *demo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
