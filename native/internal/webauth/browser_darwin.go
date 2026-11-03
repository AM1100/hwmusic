//go:build darwin && cgo

// Package webauth uses the OS authorization browser without a global URL scheme.
package webauth

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AuthenticationServices
#include <stdint.h>
#include <stdlib.h>
void *hm_auth_start(const char *url, uintptr_t handle);
void hm_auth_close(void *session);
*/
import "C"

import (
	"context"
	"errors"
	"runtime/cgo"
	"unsafe"

	"hwmusic/native/internal/huawei"
)

type result struct {
	url    string
	status int
}

// Authorize must run while Viva's native event loop is running and its main
// window is visible. Call from a worker; the OS UI is dispatched to the main
// thread. No Cookie, password or account token crosses this interface.
func Authorize(ctx context.Context, attempt *huawei.Attempt) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.New("authorization cancelled")
	}
	ch := make(chan result, 1)
	handle := cgo.NewHandle(ch)
	raw := C.CString(attempt.AuthorizeURL())
	session := C.hm_auth_start(raw, C.uintptr_t(handle))
	C.free(unsafe.Pointer(raw))
	defer C.hm_auth_close(session)
	select {
	case r := <-ch:
		if r.status != 0 {
			return "", errors.New("system browser authorization cancelled or unavailable")
		}
		if !huawei.IsCallback(r.url) {
			return "", errors.New("system browser callback rejected")
		}
		return r.url, nil
	case <-ctx.Done():
		return "", errors.New("authorization cancelled")
	}
}

//export hm_auth_complete
func hm_auth_complete(handle C.uintptr_t, raw *C.char, status C.int) {
	h := cgo.Handle(handle)
	ch := h.Value().(chan result)
	value := ""
	if raw != nil {
		value = C.GoString(raw)
	}
	ch <- result{url: value, status: int(status)}
	h.Delete()
}
