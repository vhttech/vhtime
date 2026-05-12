package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"vhtime/config"
	"vhtime/ui"

	"vhtime/bamboo-core"
	ibus "vhtime/goibus"
	"github.com/godbus/dbus/v5"
)

const KeypressDelayMs = 10

// GetIBusEngineCreator returns the factory function IBus calls when a new engine
// instance is needed. Each call produces an independent Engine bound to its own
// D-Bus object path.
func GetIBusEngineCreator() func(*dbus.Conn, string) dbus.ObjectPath {
	return func(conn *dbus.Conn, ngName string) dbus.ObjectPath {
		var ngGroupName = strings.Split(ngName, "::")[0]
		var engineName = strings.ToLower(ngGroupName)
		var cfg = config.LoadConfig(engineName)
		var objectPath = dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/IBus/Engine/%s/%d", engineName, time.Now().UnixNano()))
		var inputMethod = bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
		baseEngine := ibus.BaseEngine(conn, objectPath)
		var engine = NewIbusBambooEngine(engineName, cfg, &baseEngine, bamboo.NewEngine(inputMethod, cfg.Flags))
		engine.propList = GetPropListByConfig(cfg)
		engine.shouldEnqueueKeyStrokes = true
		engine.openGUI = ui.OpenGUI
		ibus.PublishEngine(conn, objectPath, engine)
		go engine.init()
		return objectPath
	}
}

func (e *Engine) init() {
	initConfigFiles(e.engineName)
	e.emoji = NewEmojiEngine()
	if e.macroTable == nil {
		e.macroTable = NewMacroTable(e.config.IBflags&config.IBautoCapitalizeMacro != 0)
		if e.config.IBflags&config.IBmacroEnabled != 0 {
			e.macroTable.Enable(e.engineName)
		}
	}
	e.keyPressHandler = e.forwardOrDropKeyPress
	e.startKeyPressCapturing()

	// Mouse capturing uses X11 Record extension and XTest — both unavailable on
	// pure Wayland. Guard to avoid silent no-ops that waste cycles and confuse state.
	if !isWayland && e.config.IBflags&config.IBmouseCapturing != 0 {
		startMouseCapturing()
		startMouseRecording()
	}
	var mouseMutex sync.Mutex
	onMouseMove = func() {
		mouseMutex.Lock()
		defer mouseMutex.Unlock()
		if e.checkInputMode(config.PreeditIM) && e.rawInputLen() > 0 {
			e.commitPreeditAndReset(e.getPreeditString())
		}
	}
	onMouseClick = func() {
		mouseMutex.Lock()
		defer mouseMutex.Unlock()
		if e.isEmojiLTOpened {
			e.refreshEmojiCandidate()
		} else {
			e.resetFakeBackspace()
			e.resetBuffer()
			e.keyPressDelay = KeypressDelayMs
			// x11SendShiftR triggers SurroundingText retrieval via XTest.
			// On Wayland this is unavailable — skip and rely on IBus surrounding-text
			// signal directly (isSurroundingTextReady set by FocusIn/RequireSurroundingText).
			if !isWayland && e.capabilities&IBusCapSurroundingText != 0 {
				x11SendShiftR()
				e.isSurroundingTextReady = true
				e.keyPressDelay = KeypressDelayMs * 10
			}
		}
	}
}

func initConfigFiles(engineName string) {
	if sta, err := os.Stat(config.GetConfigDir(engineName)); err != nil || !sta.IsDir() {
		if err = os.Mkdir(config.GetConfigDir(engineName), 0777); err != nil {
			panic(err)
		}
	}
	macroPath := config.GetMacroPath(engineName)
	if _, err := os.Stat(macroPath); os.IsNotExist(err) {
		sampleFile := getEngineSubFile(sampleMactabFile)
		sample, err := os.ReadFile(sampleFile)
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(macroPath, sample, 0644); err != nil {
			panic(err)
		}
	}
}

func (e *Engine) startKeyPressCapturing() {
	go func() {
		for keyEvents := range e.keyPressChan {
			atomic.StoreInt32(&e.lenKeyChan, int32(len(e.keyPressChan)))
			e.keyPressHandler(keyEvents[0], keyEvents[1], keyEvents[2])
			atomic.AddInt32(&e.lenKeyChan, -1)
		}
	}()
}

func (e *Engine) waitForKeyPressQueue() {
	for i := 0; i < 10 && atomic.LoadInt32(&e.lenKeyChan) > 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
}
