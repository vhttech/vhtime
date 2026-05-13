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
	"vhtime/config"
	"log"
	"strings"
	"time"

	"vhtime/bamboo-core"
	ibus "vhtime/goibus"
	"github.com/godbus/dbus/v5"
)

func (e *Engine) preeditProcessKeyEvent(keyVal uint32, keyCode uint32, state uint32) (bool, *dbus.Error) {
	var rawKeyLen = e.rawInputLen()
	var keyRune = rune(keyVal)
	var oldText = e.getPreeditString()
	defer e.updateLastKeyWithShift(keyVal, state)

	if !e.shouldRestoreKeyStrokes {
		if !e.preeditor.CanProcessKey(keyRune) && rawKeyLen == 0 && e.config.IBflags&config.IBmacroEnabled == 0 {
			// don't process special characters if rawKeyLen == 0,
			// workaround for Chrome's address bar and Google SpreadSheets
			return false, nil
		}
	}

	if keyVal == IBusBackSpace {
		if e.runeCount() == 1 {
			e.commitPreeditAndReset("")
			return true, nil
		}
		if rawKeyLen > 0 {
			e.preeditor.RemoveLastChar(true)
			e.updatePreedit(e.getPreeditString())
			return true, nil
		} else {
			return false, nil
		}
	}
	if keyVal == IBusTab {
		if ok, macText := e.getMacroText(); ok {
			e.commitPreeditAndReset(macText)
		} else {
			e.commitPreeditAndReset(e.getComposedString(oldText))
			return false, nil
		}
		return true, nil
	}

	// Address-bar autocomplete fix for browsers (X11 + Wayland).
	//
	// Problem: when rawKeyLen==1 (e.g. the user typed "fa" where 'f' was
	// passed natively and 'a' is the sole preedit char), IBus put Chrome into
	// composition mode. Chrome's omnibox disables inline autocomplete while
	// composition is active, so pressing Enter searches for "fa" instead of
	// navigating to facebook.com — even though the popup shows the suggestion.
	//
	// Fix: instead of CommitText (compositionend → async autocomplete re-query
	// → race with Enter), we:
	//   1. Clear the preedit → Chrome exits composition mode.
	//   2. ForwardKeyEvent(char) → Chrome processes the char as a native key
	//      event, immediately updating the inline autocomplete suggestion.
	//   3. return false → Chrome receives Enter natively AFTER the char and
	//      navigates to the top autocomplete suggestion.
	//
	// Ordering guarantee: ForwardKeyEvent is sent during ProcessKeyEvent
	// processing, before return false triggers re-injection of Enter. On both
	// X11 (X server queue) and Wayland (compositor queue) the char event
	// arrives at Chrome's window before the re-injected Enter event.
	if (keyVal == IBusReturn || keyVal == 0xff8d) && rawKeyLen == 1 && e.inBrowserList() {
		if preeditRunes := []rune(oldText); len(preeditRunes) == 1 {
			e.UpdatePreeditText(ibus.NewText(""), 0, false)
			e.HidePreeditText()
			e.HideAuxiliaryText()
			e.preeditor.Reset()
			kv := vnSymMapping[preeditRunes[0]]
			if kv == 0 {
				kv = uint32(preeditRunes[0])
			}
			e.ForwardKeyEvent(kv, 0, 0)
			e.ForwardKeyEvent(kv, 0, IBusReleaseMask)
			return false, nil
		}
	}

	newText, isWordBreakRune := e.getCommitText(keyVal, keyCode, state)
	isPrintableKey := e.isPrintableKey(state, keyVal)
	if isWordBreakRune {
		e.commitPreeditAndResetForWBS(newText, isPrintableKey)
		return isPrintableKey, nil
	}
	e.updatePreedit(newText)
	return isPrintableKey, nil
}

func (e *Engine) expandMacro(str string) string {
	var macroText = e.macroTable.GetText(str)
	if e.config.IBflags&config.IBautoCapitalizeMacro != 0 {
		switch determineMacroCase(str) {
		case VnCaseAllSmall:
			return strings.ToLower(macroText)
		case VnCaseAllCapital:
			return strings.ToUpper(macroText)
		}
	}
	return macroText
}

func (e *Engine) updatePreedit(processedStr string) {
	var encodedStr = e.encodeText(processedStr)
	var preeditLen = uint32(len([]rune(encodedStr)))
	// Signal mouse capture only when preedit is non-empty.  Signalling on empty
	// preedit (e.g. after backspace clears the buffer) would cause the grab thread
	// to re-grab the pointer with nothing to commit, blocking user clicks.
	defer func() {
		if e.config.IBflags&config.IBmouseCapturing != 0 && preeditLen > 0 {
			mouseCaptureUnlock()
		}
	}()
	if preeditLen == 0 {
		e.HidePreeditText()
		e.HideAuxiliaryText()
		e.CommitText(ibus.NewText(""))
		return
	}
	var ibusText = ibus.NewText(encodedStr)
	if inStringList(enabledAuxiliaryTextList, e.getWmClass()) && e.config.IBflags&config.IBworkaroundForWPS != 0 {
		e.UpdateAuxiliaryText(ibusText, true)
		return
	}

	if e.config.IBflags&config.IBnoUnderline == 0 {
		ibusText.AppendAttr(ibus.IBUS_ATTR_TYPE_UNDERLINE, ibus.IBUS_ATTR_UNDERLINE_SINGLE, 0, preeditLen)
	}
	// Use PREEDIT_CLEAR: FocusOut now explicitly commits via resetBuffer(), so
	// we no longer need the client to auto-commit on reset. PREEDIT_COMMIT was the
	// root cause of the "ghost word" bug where Chromium/Electron re-inserted the
	// last word into a cleared textbox after a form was submitted.
	e.UpdatePreeditTextWithMode(ibusText, preeditLen, true, ibus.IBUS_ENGINE_PREEDIT_CLEAR)
}

func (e *Engine) getBambooInputMode() bamboo.Mode {
	if e.shouldFallbackToEnglish(false) {
		return bamboo.EnglishMode
	}
	return bamboo.VietnameseMode
}

func (e *Engine) shouldFallbackToEnglish(checkVnRune bool) bool {
	if e.config.IBflags&config.IBautoNonVnRestore == 0 {
		return false
	}
	var vnSeq = e.getProcessedString(bamboo.VietnameseMode | bamboo.LowerCase)
	var vnRunes = []rune(vnSeq)
	if len(vnRunes) == 0 {
		return false
	}
	if ok, _ := e.getMacroText(); ok {
		return false
	}
	// we want to allow dd even in non-vn sequence, because dd is used a lot in abbreviation
	if e.config.IBflags&config.IBddFreeStyle != 0 && !bamboo.HasAnyVietnameseVower(vnSeq) &&
		(vnRunes[len(vnRunes)-1] == 'd' || strings.ContainsRune(vnSeq, 'đ')) {
		return false
	}
	if checkVnRune && !bamboo.HasAnyVietnameseRune(vnSeq) {
		return false
	}
	return !e.preeditor.IsValid(false)
}

func (e *Engine) mustFallbackToEnglish() bool {
	if e.config.IBflags&config.IBautoNonVnRestore == 0 {
		return false
	}
	var vnSeq = e.getProcessedString(bamboo.VietnameseMode | bamboo.LowerCase)
	var vnRunes = []rune(vnSeq)
	if len(vnRunes) == 0 {
		return false
	}
	// we want to allow dd even in non-vn sequence, because dd is used a lot in abbreviation
	if e.config.IBflags&config.IBddFreeStyle != 0 && strings.ContainsRune(vnSeq, 'đ') {
		return false
	}
	if e.config.IBflags&config.IBspellCheckWithDicts != 0 {
		return !e.dictionary[vnSeq]
	}
	return !e.preeditor.IsValid(true)
}

func (e *Engine) getComposedString(oldText string) string {
	if bamboo.HasAnyVietnameseRune(oldText) && e.mustFallbackToEnglish() {
		return e.getProcessedString(bamboo.EnglishMode)
	}
	return oldText
}

func (e *Engine) encodeText(text string) string {
	return bamboo.Encode(e.config.OutputCharset, text)
}

func (e *Engine) getProcessedString(mode bamboo.Mode) string {
	return e.preeditor.GetProcessedString(mode)
}

func (e *Engine) getPreeditString() string {
	if e.config.IBflags&config.IBmacroEnabled != 0 {
		return e.getProcessedString(bamboo.PunctuationMode)
	}
	if e.shouldFallbackToEnglish(true) {
		return e.getProcessedString(bamboo.EnglishMode)
	}
	return e.getProcessedString(bamboo.VietnameseMode)
}

func (e *Engine) resetPreedit() {
	e.HidePreeditText()
	e.HideAuxiliaryText()
	e.preeditor.Reset()
}

func (e *Engine) commitPreeditAndResetForWBS(s string, isPrintable bool) {
	// CommitText must fire BEFORE clearing the preedit.
	// Clearing first (UpdatePreeditText("")) causes Chrome to fire compositionend("")
	// which React/Draft.js/Lexical interprets as a cancelled composition and reverts
	// the input to its pre-composition state — making the word disappear. Committing
	// first lets the browser fire compositionend(s) atomically with the final text.
	// PREEDIT_CLEAR (set in updatePreedit) prevents the browser from auto-committing
	// the preedit again on IBus reset, so there is no double-commit risk.
	e.commitText(s)
	e.UpdatePreeditText(ibus.NewText(""), 0, false)
	e.HidePreeditText()
	e.HideAuxiliaryText()
	e.HideLookupTable()
	e.preeditor.Reset()
}

func (e *Engine) commitPreeditAndReset(s string) {
	e.commitText(s)
	e.UpdatePreeditText(ibus.NewText(""), 0, false)
	e.HidePreeditText()
	e.HideAuxiliaryText()
	e.HideLookupTable()
	e.preeditor.Reset()
}

func (e *Engine) commitText(str string) {
	if str == "" {
		return
	}
	log.Printf("Commit Text [%s]\n", str)
	var now = time.Now()
	e.lastCommitText = now.UnixNano()
	e.CommitText(ibus.NewText(e.encodeText(str)))
}

func (e *Engine) getVnSeq() string {
	return e.preeditor.GetProcessedString(bamboo.VietnameseMode)
}

func (e *Engine) hasMacroKey(key string) bool {
	return e.macroTable.GetText(key) != ""
}
