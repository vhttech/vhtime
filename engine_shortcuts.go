package main

import (
	"unicode"

	"vhtime/config"
)

func (e *Engine) isShortcutKeyEnable(ski uint) bool {
	if int(ski+2) > len(e.config.Shortcuts) {
		return false
	}
	return e.config.Shortcuts[ski+1] > 0
}

func (e *Engine) isShortcutKeyPressed(keyVal, state uint32, shortcut uint) bool {
	if !e.isShortcutKeyEnable(shortcut) {
		return false
	}
	realState := state & IBusDefaultModMask
	lowerKey := uint32(unicode.ToLower(rune(keyVal)))
	shortcuts := e.config.Shortcuts[shortcut : shortcut+2]
	ret := shortcuts[0] == realState && shortcuts[1] == lowerKey
	// Shift-only press must not be treated as a Vi/En switch to avoid false triggers
	// when the user holds Shift to type uppercase letters.
	if realState == IBusShiftMask && shortcut == KSViEnSwitch {
		return ret && !e.lastKeyWithShift
	}
	return ret
}

// processShortcutKey handles all global shortcut keys before normal key dispatch.
// Returns (handled bool, consumed bool) — handled=true means the key was intercepted;
// consumed=true means the caller should return true to IBus (key was used).
func (e *Engine) processShortcutKey(keyVal, keyCode, state uint32) (handled, consumed bool) {
	if keyVal == IBusCapsLock {
		return true, false
	}

	// Emoji picker
	if e.isShortcutKeyPressed(keyVal, state, KSEmojiDialog) && !e.isEmojiLTOpened {
		e.resetBuffer()
		e.isEmojiLTOpened = true
		e.lastKeyWithShift = true
		e.openEmojiList()
		return true, true
	}
	if e.isEmojiLTOpened {
		return true, e.emojiProcessKeyEvent(keyVal, keyCode, state)
	}

	// Hex Unicode input
	if e.isShortcutKeyPressed(keyVal, state, KSHexadecimal) {
		e.resetBuffer()
		e.isInHexadecimal = true
		e.setupHexadecimalProcessKeyEvent()
		return true, true
	}
	if e.isInHexadecimal {
		if e.isShortcutKeyPressed(keyVal, state, KSHexadecimal) {
			e.closeHexadecimalInput()
			e.updateLastKeyWithShift(keyVal, state)
			return true, false
		}
		return true, e.hexadecimalProcessKeyEvent(keyVal, keyCode, state)
	}

	// US-only mode: block all Vietnamese processing
	if e.config.DefaultInputMode == config.UsIM {
		return true, false
	}

	// Restore raw key strokes
	if e.isShortcutKeyPressed(keyVal, state, KSRestoreKeyStrokes) {
		e.shouldRestoreKeyStrokes = true
		return false, false
	}

	// Vi/En language toggle
	if e.isShortcutKeyPressed(keyVal, state, KSViEnSwitch) {
		e.englishMode = !e.englishMode
		notify(e.englishMode)
		e.resetBuffer()
		return true, true
	}

	// Input mode switcher lookup table
	if e.isInputModeLTOpened {
		return e.modeSwitcherKeyEvent(keyVal, keyCode, state)
	}
	if e.isShortcutKeyPressed(keyVal, state, KSInputModeSwitch) && e.getWmClass() != "" {
		e.resetBuffer()
		e.isInputModeLTOpened = true
		e.lastKeyWithShift = true
		e.openLookupTable()
		return true, true
	}

	// Let Shift pass through (used for uppercase, not a shortcut)
	if keyVal == IBusShiftL || keyVal == IBusShiftR {
		return true, false
	}

	// Per-app US mode
	if e.checkInputMode(config.UsIM) {
		return true, false
	}

	// English mode toggle
	if e.englishMode {
		e.updateLastKeyWithShift(keyVal, state)
		return true, false
	}

	return false, false
}
