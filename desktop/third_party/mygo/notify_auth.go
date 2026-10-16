package mygo

// [ai-news-radar patch] Explicit notification authorization (UNUserNotificationCenter on macOS).

import (
	"time"

	"github.com/egoist/mygo/internal/platform"
)

type notifyAuthBackend interface {
	NotificationAuthorizationStatus(done func(int))
	RequestNotificationAuthorization(done func(granted bool, err error))
}

// NotificationAuthorization statuses.
const (
	NotificationAuthUnsupported   = "unsupported"
	NotificationAuthNotDetermined = "unknown"
	NotificationAuthDenied        = "denied"
	NotificationAuthGranted       = "granted"
)

func authName(st int) string {
	switch st {
	case 0:
		return NotificationAuthNotDetermined
	case 1:
		return NotificationAuthDenied
	case 2, 3, 4:
		return NotificationAuthGranted
	}
	return NotificationAuthUnsupported
}

// NotificationAuthorizationStatus reports the system's notification permission for the app. Call it off the
// main thread (it waits up to 5 s for the system's answer).
func NotificationAuthorizationStatus() string {
	ch := make(chan int, 1)
	onMain(func() {
		b, ok := backend().(notifyAuthBackend)
		if !ok {
			ch <- -1
			return
		}
		b.NotificationAuthorizationStatus(func(st int) { ch <- st })
	})
	select {
	case st := <-ch:
		return authName(st)
	case <-time.After(5 * time.Second):
		return NotificationAuthUnsupported
	}
}

// RequestNotificationAuthorization asks the user (the system shows its prompt only while the status is not
// determined) and returns the answer. Call it off the main thread; it waits up to 2 minutes for the user.
func RequestNotificationAuthorization() (bool, error) {
	type res struct {
		ok  bool
		err error
	}
	ch := make(chan res, 1)
	onMain(func() {
		b, ok := backend().(notifyAuthBackend)
		if !ok {
			ch <- res{false, platform.ErrUnsupported}
			return
		}
		b.RequestNotificationAuthorization(func(g bool, err error) { ch <- res{g, err} })
	})
	select {
	case r := <-ch:
		return r.ok, r.err
	case <-time.After(2 * time.Minute):
		return false, nil
	}
}
