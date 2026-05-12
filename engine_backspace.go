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
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"vhtime/config"

	"vhtime/bamboo-core"
	"github.com/godbus/dbus/v5"
)

const BACKSPACE_INTERVAL = 0

func (e *Engine) backspaceProcessKeyEvent(keyVal uint32, keyCode uint32, state uint32) (bool, *dbus.Error) {
	if isMovementKey(keyVal) {
		e.preeditor.Reset()
		e.resetFakeBackspace()
		e.isSurroundingTextReady = true
		return false, nil
	}
	var keyRune = rune(keyVal)
	if e.config.IBflags&config.IBmacroEnabled == 0 && len(e.keyPressChan) == 0 && e.rawInputLen() == 0 && !inKeyList(e.preeditor.GetInputMethod().AppendingKeys, keyRune) {
		e.updateLastKeyWithShift(keyVal, state)
		if e.preeditor.CanProcessKey(keyRune) && isValidState(state) {
			e.isFirstTimeSendingBS = true
			if state&IBusLockMask != 0 {
				keyRune = e.toUpper(keyRune)
			}
			e.preeditor.ProcessKey(keyRune, bamboo.VietnameseMode)
			e.bsCommitText([]rune(e.getPreeditString()))
			return true, nil
		}
		return false, nil
	}

	if e.shouldEnqueueKeyStrokes {
		// WARNING: don't use ForwardKeyEvent api in XTestFakeKeyEvent/SurroundingText mode
		if e.checkInputMode(config.XTestFakeKeyEventIM) || e.checkInputMode(config.SurroundingTextIM) {
			if keyVal == IBusBackSpace {
				if e.getFakeBackspace() > 0 {
					e.addFakeBackspace(-1)
					return false, nil
				} else {
					e.waitForKeyPressQueue()
					if e.rawInputLen() > 0 {
						if e.shouldFallbackToEnglish(true) {
							e.preeditor.RestoreLastWord(false)
						}
						e.preeditor.RemoveLastChar(false)
					}
				}
				return false, nil
			}
			if keyVal == IBusTab {
				e.waitForKeyPressQueue()
				if ok, _ := e.getMacroText(); !ok {
					e.preeditor.Reset()
					return false, nil
				}
			}
			isValidKey := isValidState(state) && e.isValidKeyVal(keyVal)
			if !isValidKey {
				e.waitForKeyPressQueue()
				return e.processKeyPress(keyVal, keyCode, state), nil
			}
		}
		// if the main thread is busy processing, the keypress events come all mixed up
		// so we enqueue these keypress events and process them sequentially on another thread
		e.keyPressChan <- [3]uint32{keyVal, keyCode, state}
		return true, nil
	} else {
		return e.processKeyPress(keyVal, keyCode, state), nil
	}
}

func (e *Engine) forwardOrDropKeyPress(keyVal, keyCode, state uint32) {
	ret := e.processKeyPress(keyVal, keyCode, state)
	if !ret {
		e.ForwardKeyEvent(keyVal, keyCode, state)
	}
}

func (e *Engine) processKeyPress(keyVal, keyCode, state uint32) bool {
	defer e.updateLastKeyWithShift(keyVal, state)
	if e.keyPressDelay > 0 {
		time.Sleep(time.Duration(e.keyPressDelay) * time.Millisecond)
		e.keyPressDelay = 0
	}
	oldText := e.getPreeditString()
	_, oldMacText := e.getMacroText()
	if keyVal == IBusBackSpace {
		if e.rawInputLen() > 0 {
			if e.config.IBflags&config.IBautoNonVnRestore == 0 {
				e.preeditor.RemoveLastChar(false)
				return false
			}
			e.preeditor.RemoveLastChar(true)
			var newText = e.getPreeditString()
			var offset = e.getPreeditOffset([]rune(newText), []rune(oldText))
			if oldText != "" && offset != len([]rune(newText)) {
				e.updatePreviousText(oldText, newText)
				return true
			}
		}
		return false
	}

	if keyVal == IBusTab {
		defer e.preeditor.Reset()
		if oldMacText != "" {
			e.updatePreviousText(oldText, oldMacText)
			return true
		}
		return false
	}

	isValidKey := isValidState(state) && e.isValidKeyVal(keyVal)
	newText, isWordBreakRune := e.getCommitText(keyVal, keyCode, state)
	if len(newText) > 0 {
		if e.shouldAppendDeadKey(newText, oldText) {
			e.bsCommitText([]rune(" "))
			time.Sleep(10 * time.Millisecond)
			e.isFirstTimeSendingBS = false
			e.SendBackSpace(1)
		}
		e.updatePreviousTextInBatch(oldText, newText, isWordBreakRune)
		return isValidKey
	}
	return isValidKey
}

func (e *Engine) getPreeditOffset(newRunes, oldRunes []rune) int {
	var minLen = len(oldRunes)
	if len(newRunes) < minLen {
		minLen = len(newRunes)
	}
	for i := 0; i < minLen; i++ {
		if oldRunes[i] != newRunes[i] {
			return i
		}
	}
	return minLen
}

func (e *Engine) shouldAppendDeadKey(newText, oldText string) bool {
	var oldRunes = []rune(oldText)
	var newRunes = []rune(newText)
	var offset = e.getPreeditOffset(newRunes, oldRunes)

	// workaround for chrome and firefox's address bar
	if e.isFirstTimeSendingBS && offset < len(newRunes) && offset < len(oldRunes) && e.inBrowserList() &&
		!e.checkInputMode(config.ShiftLeftForwardingIM) {
		return true
	}
	return false
}

func (e *Engine) updatePreviousText(oldText, newText string) {
	offsetRunes, nBackSpace := e.getOffsetRunes(newText, oldText)
	if nBackSpace > 0 {
		e.SendBackSpace(nBackSpace)
	}
	log.Printf("Updating Previous Text %s ---> %s\n", oldText, newText)
	e.bsCommitText(offsetRunes)
}

func (e *Engine) updatePreviousTextInBatch(oldText, newText string, isWordBreakRune bool) {
	offsetRunes, nBackSpace := e.getOffsetRunes(newText, oldText)
	if nBackSpace > 0 {
		e.SendBackSpace(nBackSpace)
	}
	var buffer = []string{string(offsetRunes)}
	if isWordBreakRune {
		e.preeditor.Reset()
		buffer = append(buffer, "")
	}
	// isDirty means containing runes that are not committed
	var isDirty = false
	for i := 0; i < len(e.keyPressChan); i++ {
		var keyEvents = <-e.keyPressChan
		var keyVal, keyCode, state = keyEvents[0], keyEvents[1], keyEvents[2]
		isValidKey := isValidState(state) && e.isValidKeyVal(keyVal)
		if isValidKey {
			var commitText, isWordBreakRune0 = e.getCommitText(keyVal, keyCode, state)
			buffer[len(buffer)-1] = commitText
			if isWordBreakRune0 {
				buffer = append(buffer, "")
			}
			isDirty = true
		} else {
			if isDirty {
				e.batchCommit(oldText, strings.Join(buffer, ""), nBackSpace, isWordBreakRune)
				buffer = []string{""}
			}
			e.ForwardKeyEvent(keyVal, keyCode, state)
		}
	}
	if isDirty {
		e.batchCommit(oldText, strings.Join(buffer, ""), nBackSpace, isWordBreakRune)
		return
	}
	log.Printf("Updating Previous Text %s ---> %s\n", oldText, newText)
	e.bsCommitText(offsetRunes)
}

// batchCommit compares two given text and commit the right outer text, with backspaces if necessary
// toi - tôi = ôi + 2 BS
// <space> - tôi = tôi
func (e *Engine) batchCommit(oldText string, newText string, nBackSpace int, isWordBreakRune bool) {
	fullRunes := []rune(newText)
	if len(fullRunes) == 0 {
		return
	}
	patchedRunes, patchedBackSpace := e.getOffsetRunes(newText, oldText)
	if isWordBreakRune {
		e.bsCommitText(patchedRunes)
		return
	}
	if patchedBackSpace > nBackSpace {
		e.SendBackSpace(patchedBackSpace - nBackSpace)
	} else if patchedBackSpace < nBackSpace {
		var offset = utf8.RuneCountInString(oldText) - nBackSpace
		patchedRunes = fullRunes[offset:]
	}
	e.bsCommitText(patchedRunes)
}

// getOffsetRunes returns the right outer text and number of pending backspaces
func (e *Engine) getOffsetRunes(newText, oldText string) ([]rune, int) {
	var oldRunes = []rune(oldText)
	var newRunes = []rune(newText)
	var nBackSpace = 0
	var offset = e.getPreeditOffset(newRunes, oldRunes)
	if offset < len(oldRunes) {
		nBackSpace += len(oldRunes) - offset
	}

	return newRunes[offset:], nBackSpace
}

// SendBackSpace erases n characters via the mechanism appropriate for the current
// input mode. Timing delays are intentional — GTK/Qt have a sync issue between
// fake backspaces and committed text; removing or shortening the delays causes
// dropped or mis-ordered characters.
func (e *Engine) SendBackSpace(n int) {
	// Wait until at least 50 ms have passed since the last CommitText to avoid
	// the GTK/Qt sync issue.
	if delta := 50*1000*1000 - (time.Now().UnixNano() - e.lastCommitText); delta > 0 {
		time.Sleep(time.Duration(delta) * time.Nanosecond)
	}

	mode := e.getInputMode()
	log.Printf("SendBackSpace: n=%d mode=%d\n", n, mode)

	switch mode {
	case config.XTestFakeKeyEventIM:
		e.setFakeBackspace(int32(n))
		time.Sleep(10 * time.Millisecond)
		x11SendBackspace(n, 0)
		// Poll until XTest events have been processed by the application.
		for count := 0; e.getFakeBackspace() > 0 && count < 10; count++ {
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(time.Duration(n) * (10 + BACKSPACE_INTERVAL) * time.Millisecond)

	case config.SurroundingTextIM:
		time.Sleep(20 * time.Millisecond)
		e.DeleteSurroundingText(-int32(n), uint32(n))
		time.Sleep(20 * time.Millisecond)

	case config.ForwardAsCommitIM:
		time.Sleep(20 * time.Millisecond)
		for i := 0; i < n; i++ {
			e.ForwardKeyEvent(IBusBackSpace, XkBackspace-8, 0)
			e.ForwardKeyEvent(IBusBackSpace, XkBackspace-8, IBusReleaseMask)
		}
		time.Sleep(time.Duration(n) * (20 + BACKSPACE_INTERVAL) * time.Millisecond)

	case config.ShiftLeftForwardingIM:
		time.Sleep(30 * time.Millisecond)
		for i := 0; i < n; i++ {
			e.ForwardKeyEvent(IBusLeft, XkLeft-8, IBusShiftMask)
			e.ForwardKeyEvent(IBusLeft, XkLeft-8, IBusReleaseMask)
		}
		time.Sleep(time.Duration(n) * (30 + BACKSPACE_INTERVAL) * time.Millisecond)

	case config.BackspaceForwardingIM:
		time.Sleep(30 * time.Millisecond)
		for i := 0; i < n; i++ {
			e.ForwardKeyEvent(IBusBackSpace, XkBackspace-8, 0)
			e.ForwardKeyEvent(IBusBackSpace, XkBackspace-8, IBusReleaseMask)
		}
		time.Sleep(time.Duration(n) * (30 + BACKSPACE_INTERVAL) * time.Millisecond)

	default:
		log.Println("SendBackSpace: unknown input mode, wmClasses may be empty")
	}
}

func (e *Engine) resetFakeBackspace() {
	e.setFakeBackspace(0)
}

func (e *Engine) bsCommitText(rs []rune) {
	if len(rs) == 0 {
		return
	}
	if e.checkInputMode(config.ForwardAsCommitIM) {
		log.Println("Forward as commit", string(rs))
		for _, chr := range rs {
			var keyVal = vnSymMapping[chr]
			if keyVal == 0 {
				keyVal = uint32(chr)
			}
			e.ForwardKeyEvent(keyVal, 0, 0)
			e.ForwardKeyEvent(keyVal, 0, IBusReleaseMask)
		}
		time.Sleep(time.Duration(len(rs)) * 5 * time.Millisecond)
		return
	}
	e.commitText(string(rs))
}
