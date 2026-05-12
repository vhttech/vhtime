package main

import (
	"fmt"
	"strings"

	"vhtime/config"

	"github.com/godbus/dbus/v5"
)

func (e *Engine) getWmClass() string {
	return e.wmClasses
}

// getLatestWmClass queries the currently focused window class from the best
// available source: GNOME Shell D-Bus → Wayland foreign-toplevel → X11.
func (e *Engine) getLatestWmClass() string {
	var wmClass string
	if isGnome {
		wmClass, _ = gnomeGetFocusWindowClass()
	} else if isWayland {
		wmClass = getWlAppId()
	}
	if wmClass == "" {
		wmClass = x11GetFocusWindowClass()
	}
	return strings.ReplaceAll(wmClass, "\"", "")
}

func (e *Engine) checkWmClass(newId string) {
	if e.wmClasses != newId {
		e.wmClasses = newId
		e.resetBuffer()
		e.resetFakeBackspace()
	}
}

func (e *Engine) inBrowserList() bool {
	return inStringList(DefaultBrowserList, e.getWmClass())
}

func (e *Engine) resetBuffer() {
	if e.rawInputLen() == 0 {
		return
	}
	if e.checkInputMode(config.PreeditIM) {
		e.commitPreeditAndReset(e.getPreeditString())
	} else {
		e.preeditor.Reset()
	}
}

// notify sends a desktop notification when the user switches Vi/En mode.
func notify(enMode bool) {
	title := "Vietnamese"
	if enMode {
		title = "English"
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Println(err)
		return
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(281025),
		"", title, "Press Shortcut keys to switch input language",
		[]string{}, map[string]dbus.Variant{}, int32(3000))
	if call.Err != nil {
		fmt.Println(call.Err)
	}
}
