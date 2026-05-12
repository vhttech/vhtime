package main

import (
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
)

// gnomeShellEvalWarned ensures the "Shell.Eval restricted" warning is logged once.
var gnomeShellEvalWarned sync.Once

// gnomeGetFocusWindowClass returns the wm_class of the currently focused window
// on GNOME by calling org.gnome.Shell.Eval with a JavaScript snippet.
//
// Limitation: GNOME 41+ restricts Shell.Eval to trusted extensions by default.
// When restricted, ok=false and the call returns an empty string. In that case
// we return ("", nil) and the caller falls back to X11 (XWayland) detection.
// Per-app input mode mapping will not work on GNOME 41+ Wayland without a
// companion GNOME Shell extension that exposes window class via a D-Bus method.
func gnomeGetFocusWindowClass() (string, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	obj := conn.Object("org.gnome.Shell", "/org/gnome/Shell")

	// Try the focused-window query first.
	const jsGetClass = `global.get_window_actors()` +
		`.find(w => !Main.overview.visible && w.meta_window.has_focus())` +
		`?.get_meta_window()?.get_wm_class() ?? ""`
	var (
		ok  bool
		cls string
	)
	if err = obj.Call("org.gnome.Shell.Eval", 0, jsGetClass).Store(&ok, &cls); err == nil && ok {
		return cls, nil
	}

	// Shell.Eval returned ok=false — restricted on GNOME 41+ without extension.
	gnomeShellEvalWarned.Do(func() {
		log.Println("vhtime: org.gnome.Shell.Eval is restricted (GNOME 41+). " +
			"Per-app input mode mapping is unavailable on GNOME Wayland. " +
			"Install the vhtime GNOME Shell extension to enable it.")
	})

	// Fall back: check if the GNOME overview is visible.
	if gnomeIsOverviewVisible(obj) {
		return "org.gnome.Overview", nil
	}
	return "", nil
}

func gnomeIsOverviewVisible(obj dbus.BusObject) bool {
	var ok bool
	var visible string
	err := obj.Call("org.gnome.Shell.Eval", 0, "Main.overview.visible").Store(&ok, &visible)
	return err == nil && ok && visible == "true"
}
