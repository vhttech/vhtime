/*
 * vhtime - A Vietnamese Input method editor
 * Copyright (C) 2018 Luong Thanh Lam <ltlam93@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"sync"

	"vhtime/config"

	"vhtime/bamboo-core"
	ibus "vhtime/goibus"
	"github.com/godbus/dbus/v5"
)

type Engine struct {
	sync.Mutex
	IEngine
	preeditor              bamboo.IEngine
	engineName             string
	config                 *config.Config
	propList               *ibus.PropList
	englishMode            bool
	macroTable             *MacroTable
	wmClasses              string
	isInputModeLTOpened    bool
	isEmojiLTOpened        bool
	isInHexadecimal        bool
	emojiLookupTable       *ibus.LookupTable
	inputModeLookupTable   *ibus.LookupTable
	capabilities           uint32
	keyPressDelay          int
	nFakeBackSpace         int32
	isFirstTimeSendingBS   bool
	emoji                  *EmojiEngine
	isSurroundingTextReady bool
	lastKeyWithShift       bool
	lastCommitText         int64
	shouldRestoreKeyStrokes bool
	shouldEnqueueKeyStrokes bool
	// openGUI is injected by the factory so the engine does not import the ui package directly.
	openGUI func(engineName string)
	// per-engine lazy-loaded resources (replacing package-level globals)
	dictionary     map[string]bool
	dictOnce       sync.Once
	spellEmojiTrie *TrieNode
	emojiOnce      sync.Once
	// key-press queue for backspace-forwarding modes (replacing package-level globals)
	keyPressChan    chan [3]uint32
	keyPressHandler func(keyVal, keyCode, state uint32)
	lenKeyChan      int32
}

func NewIbusBambooEngine(name string, cfg *config.Config, base IEngine, preeditor bamboo.IEngine) *Engine {
	e := &Engine{
		engineName:   name,
		IEngine:      base,
		preeditor:    preeditor,
		config:       cfg,
		keyPressChan: make(chan [3]uint32, 100),
	}
	return e
}

/*
*
Implement IBus.Engine's process_key_event default signal handler.

Args:

	keyval - The keycode, transformed through a keymap, stays the
		same for every keyboard
	keycode - Keyboard-dependant key code
	modifiers - The state of IBus.ModifierType keys like
		Shift, Control, etc.

Return:

	True - if successfully process the keyevent
	False - otherwise. The keyevent will be passed to X-Client

This function gets called whenever a key is pressed.
*/
func (e *Engine) ProcessKeyEvent(keyVal uint32, keyCode uint32, state uint32) (bool, *dbus.Error) {
	if state&IBusReleaseMask != 0 {
		return false, nil
	}
	if ret, retValue := e.processShortcutKey(keyVal, keyCode, state); ret {
		return retValue, nil
	}
	if e.inBackspaceWhiteList() {
		return e.backspaceProcessKeyEvent(keyVal, keyCode, state)
	}
	return e.preeditProcessKeyEvent(keyVal, keyCode, state)
}

func (e *Engine) FocusIn() *dbus.Error {
	var latestWm = e.getLatestWmClass()
	e.checkWmClass(latestWm)
	e.RegisterProperties(e.propList)
	e.RequireSurroundingText()
	if e.isShortcutKeyEnable(KSEmojiDialog) {
		e.emojiOnce.Do(func() {
			var err error
			e.spellEmojiTrie, err = loadEmojiOne(DictEmojiOne)
			if err != nil {
				panic(fmt.Sprintf("failed to load emoji trie from %s: %s", DictEmojiOne, err))
			}
		})
	}
	if e.config.IBflags&config.IBspellCheckWithDicts != 0 {
		e.dictOnce.Do(func() {
			e.dictionary, _ = loadDictionary(DictVietnameseCm)
		})
	}
	if !isWayland {
		if inStringList(disabledMouseCapturingList, e.getWmClass()) {
			stopMouseCapturing()
		} else if e.config.IBflags&config.IBmouseCapturing != 0 {
			startMouseCapturing()
		}
	}
	return nil
}

func (e *Engine) FocusOut() *dbus.Error {
	return nil
}

func (e *Engine) Reset() *dbus.Error {
	if e.checkInputMode(config.PreeditIM) {
		e.commitPreeditAndReset(e.getPreeditString())
	}
	return nil
}

func (e *Engine) Enable() *dbus.Error {
	e.RequireSurroundingText()
	return nil
}

func (e *Engine) Disable() *dbus.Error {
	return nil
}

// @method(in_signature="vuu")
func (e *Engine) SetSurroundingText(text dbus.Variant, cursorPos uint32, anchorPos uint32) *dbus.Error {
	if !e.isSurroundingTextReady {
		return nil
	}
	e.Lock()
	defer func() {
		e.Unlock()
		e.isSurroundingTextReady = false
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	if e.inBackspaceWhiteList() {
		var str = reflect.ValueOf(reflect.ValueOf(text.Value()).Index(2).Interface()).String()
		var s = []rune(str)
		if len(s) < int(cursorPos) {
			return nil
		}
		var cs = s[:cursorPos]
		e.preeditor.Reset()
		for i := len(cs) - 1; i >= 0; i-- {
			// workaround for spell checking
			if bamboo.IsPunctuationMark(cs[i]) && e.preeditor.CanProcessKey(cs[i]) {
				cs[i] = ' '
			}
			e.preeditor.ProcessKey(cs[i], bamboo.EnglishMode|bamboo.InReverseOrder)
		}
	}
	return nil
}

func (e *Engine) PageUp() *dbus.Error {
	if e.isEmojiLTOpened && e.emojiLookupTable.PageUp() {
		e.updateEmojiLookupTable()
	}
	if e.isInputModeLTOpened && e.inputModeLookupTable.PageUp() {
		e.updateInputModeLT()
	}
	return nil
}

func (e *Engine) PageDown() *dbus.Error {
	if e.isEmojiLTOpened && e.emojiLookupTable.PageDown() {
		e.updateEmojiLookupTable()
	}
	if e.isInputModeLTOpened && e.inputModeLookupTable.PageDown() {
		e.updateInputModeLT()
	}
	return nil
}

func (e *Engine) CursorUp() *dbus.Error {
	if e.isEmojiLTOpened && e.emojiLookupTable.CursorUp() {
		e.updateEmojiLookupTable()
	}
	if e.isInputModeLTOpened && e.inputModeLookupTable.CursorUp() {
		e.updateInputModeLT()
	}
	return nil
}

func (e *Engine) CursorDown() *dbus.Error {
	if e.isEmojiLTOpened && e.emojiLookupTable.CursorDown() {
		e.updateEmojiLookupTable()
	}
	if e.isInputModeLTOpened && e.inputModeLookupTable.CursorDown() {
		e.updateInputModeLT()
	}
	return nil
}

func (e *Engine) CandidateClicked(index uint32, button uint32, state uint32) *dbus.Error {
	if e.isEmojiLTOpened && e.updateCursorPosInEmojiTable(index) {
		e.commitEmojiCandidate()
		e.closeEmojiCandidates()
	}
	if e.isInputModeLTOpened && e.inputModeLookupTable.SetCursorPos(index) {
		e.commitInputModeCandidate()
		e.closeInputModeCandidates()
	}
	return nil
}

func (e *Engine) SetCapabilities(cap uint32) *dbus.Error {
	e.capabilities = cap
	return nil
}

func (e *Engine) SetCursorLocation(x int32, y int32, w int32, h int32) *dbus.Error {
	return nil
}

func (e *Engine) SetContentType(purpose uint32, hints uint32) *dbus.Error {
	return nil
}

// @method(in_signature="su")
func (e *Engine) PropertyActivate(propName string, propState uint32) *dbus.Error {
	// URL-opening actions — return immediately, no config save needed.
	switch propName {
	case PropKeyAbout:
		exec.Command("xdg-open", HomePage).Start()
		return nil
	case PropKeyVnCharsetConvert:
		exec.Command("xdg-open", CharsetConvertPage).Start()
		return nil
	}

	// GUI-opening actions — reload config after the dialog closes.
	switch propName {
	case PropKeyConfiguration, PropKeyInputModeLookupTableShortcut, PropKeyMacroTable:
		if e.openGUI != nil {
			e.openGUI(e.engineName)
		}
		e.config = config.LoadConfig(e.engineName)
		return nil
	}

	checked := propState == ibus.PROP_STATE_CHECKED
	e.applyPropChange(propName, checked)

	// Dynamic prop keys: input mode, output charset, input method selection.
	if im, ok := getValueFromPropKey(propName, "InputMode"); ok && checked {
		e.config.DefaultInputMode, _ = strconv.Atoi(im)
	}
	if charset, ok := getValueFromPropKey(propName, "OutputCharset"); ok && isValidCharset(charset) && checked {
		e.config.OutputCharset = charset
	}
	if _, ok := e.config.InputMethodDefinitions[propName]; ok && checked {
		e.config.InputMethod = propName
	}

	if propName != "-" {
		config.SaveConfig(e.config, e.engineName)
	}
	e.propList = GetPropListByConfig(e.config)
	e.preeditor = bamboo.NewEngine(bamboo.ParseInputMethod(e.config.InputMethodDefinitions, e.config.InputMethod), e.config.Flags)
	e.RegisterProperties(e.propList)
	return nil
}

// applyPropChange updates the in-memory config for a single boolean property toggle.
func (e *Engine) applyPropChange(propName string, checked bool) {
	setFlag := func(flag uint) {
		if checked {
			e.config.IBflags |= flag
		} else {
			e.config.IBflags &= ^flag
		}
	}
	setBambooFlag := func(flag uint) {
		if checked {
			e.config.Flags |= flag
		} else {
			e.config.Flags &= ^flag
		}
	}
	enableSpellCheck := func(on bool) {
		if on {
			e.config.IBflags |= config.IBspellCheckEnabled | config.IBautoNonVnRestore
			if e.config.IBflags&config.IBspellCheckWithDicts == 0 {
				e.config.IBflags |= config.IBspellCheckWithRules
			}
		} else {
			e.config.IBflags &= ^(config.IBspellCheckEnabled | config.IBautoNonVnRestore)
		}
	}

	switch propName {
	case PropKeyStdToneStyle:
		setBambooFlag(bamboo.EstdToneStyle)
	case PropKeyFreeToneMarking:
		setBambooFlag(bamboo.EfreeToneMarking)
	case PropKeyEnableSpellCheck:
		enableSpellCheck(checked)
	case PropKeySpellCheckByRules:
		setFlag(config.IBspellCheckWithRules)
		if checked {
			enableSpellCheck(true)
		}
	case PropKeySpellCheckByDicts:
		setFlag(config.IBspellCheckWithDicts)
		if checked {
			enableSpellCheck(true)
			e.dictionary, _ = loadDictionary(DictVietnameseCm)
		}
	case PropKeyMouseCapturing:
		setFlag(config.IBmouseCapturing)
		// X11 Record/XTest unavailable on pure Wayland — guard all calls.
		if !isWayland {
			if checked {
				startMouseCapturing()
				startMouseRecording()
			} else {
				stopMouseCapturing()
				stopMouseRecording()
			}
		}
	case PropKeyMacroEnabled:
		setFlag(config.IBmacroEnabled)
		if checked {
			e.macroTable.Enable(e.engineName)
		} else {
			e.macroTable.Disable()
		}
	case PropKeyPreeditInvisibility:
		setFlag(config.IBnoUnderline)
	case PropKeyPreeditElimination:
		setFlag(config.IBpreeditElimination)
	case PropKeyAutoCapitalizeMacro:
		setFlag(config.IBautoCapitalizeMacro)
		if e.config.IBflags&config.IBmacroEnabled != 0 {
			e.macroTable.Reload(e.engineName, checked)
		}
	}
}
