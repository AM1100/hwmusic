//go:build !linux

package terminal

import "hwmusic/native/internal/media"

func newMedia(callback func(media.Event)) *media.Controller { return media.New(callback) }
