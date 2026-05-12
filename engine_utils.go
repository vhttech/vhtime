package main

import (
	"sync/atomic"
	"unicode/utf8"

	"vhtime/config"

	"github.com/BambooEngine/bamboo-core"
)

// rawInputLen returns the number of raw (pre-composition) characters buffered
// in the preeditor. Used to distinguish "empty buffer" from "active composition".
func (e *Engine) rawInputLen() int {
	return len(e.preeditor.GetProcessedString(bamboo.EnglishMode | bamboo.FullText))
}

func (e *Engine) runeCount() int {
	return utf8.RuneCountInString(e.getPreeditString())
}

// isValidState returns true when no modifier keys that would produce non-typing
// actions (Ctrl, Alt, Super, …) are held.
func isValidState(state uint32) bool {
	return state&(IBusControlMask|IBusMod1Mask|IBusIgnoredMask|IBusSuperMask|IBusHyperMask|IBusMetaMask) == 0
}

func (e *Engine) isPrintableKey(state, keyVal uint32) bool {
	return isValidState(state) && e.isValidKeyVal(keyVal)
}

func (e *Engine) isValidKeyVal(keyVal uint32) bool {
	keyRune := rune(keyVal)
	if keyVal == IBusBackSpace || bamboo.IsWordBreakSymbol(keyRune) {
		return true
	}
	if ok, _ := e.getMacroText(); ok && keyVal == IBusTab {
		return true
	}
	return e.preeditor.CanProcessKey(keyRune)
}

// toUpper handles Caps-Lock uppercasing, with special remapping for bracket keys
// that have a Vietnamese appending-key role (e.g. [ ↔ { when Caps Lock is on).
func (e *Engine) toUpper(keyRune rune) rune {
	brackets := map[rune]rune{'[': '{', ']': '}', '{': '[', '}': ']'}
	if upper, ok := brackets[keyRune]; ok && inKeyList(e.preeditor.GetInputMethod().AppendingKeys, keyRune) {
		return upper
	}
	return keyRune
}

func (e *Engine) updateLastKeyWithShift(keyVal, state uint32) {
	if e.preeditor.CanProcessKey(rune(keyVal)) {
		e.lastKeyWithShift = state&IBusShiftMask != 0
	} else {
		e.lastKeyWithShift = false
	}
}

func (e *Engine) getMacroText() (bool, string) {
	if e.config.IBflags&config.IBmacroEnabled == 0 {
		return false, ""
	}
	text := e.preeditor.GetProcessedString(bamboo.PunctuationMode)
	if e.macroTable.HasKey(text) {
		return true, e.expandMacro(text)
	}
	return false, ""
}

func (e *Engine) getFakeBackspace() int32 {
	return atomic.LoadInt32(&e.nFakeBackSpace)
}

func (e *Engine) setFakeBackspace(n int32) {
	atomic.StoreInt32(&e.nFakeBackSpace, n)
}

func (e *Engine) addFakeBackspace(n int32) {
	atomic.AddInt32(&e.nFakeBackSpace, n)
}

// getCommitText computes the text to commit given the current key event.
// It also returns whether the key is a word-break symbol (space, punctuation, …).
// Side effect: may call preeditor.ProcessKey to advance the composition state.
func (e *Engine) getCommitText(keyVal, keyCode, state uint32) (newText string, isWordBreak bool) {
	keyRune := rune(keyVal)
	isPrintable := e.isPrintableKey(state, keyVal)
	oldText := e.getPreeditString()

	if e.shouldRestoreKeyStrokes {
		e.shouldRestoreKeyStrokes = false
		e.preeditor.RestoreLastWord(!bamboo.HasAnyVietnameseRune(oldText))
		return e.getPreeditString(), false
	}

	var keyS string
	if isPrintable {
		keyS = string(keyRune)
	}

	if isPrintable && e.preeditor.CanProcessKey(keyRune) {
		if state&IBusLockMask != 0 {
			keyRune = e.toUpper(keyRune)
		}
		e.preeditor.ProcessKey(keyRune, e.getBambooInputMode())

		if inKeyList(e.preeditor.GetInputMethod().AppendingKeys, keyRune) {
			return e.handleAppendingKey(keyRune, oldText)
		}
		if e.config.IBflags&config.IBmacroEnabled != 0 {
			return e.getProcessedString(bamboo.PunctuationMode), false
		}
		return e.getPreeditString(), false
	}

	if e.config.IBflags&config.IBmacroEnabled != 0 {
		if isPrintable && e.macroTable.HasPrefix(oldText+keyS) {
			e.preeditor.ProcessKey(keyRune, bamboo.EnglishMode)
			return oldText + keyS, false
		}
		if e.macroTable.HasKey(oldText) {
			if isPrintable {
				return e.expandMacro(oldText) + keyS, true
			}
			return e.expandMacro(oldText), true
		}
	}

	return e.handleNonVnWord(keyVal, keyCode, state), true
}

// handleAppendingKey resolves the output for keys that appear in the input method's
// AppendingKeys list (e.g. [ ] for some layouts). These keys can double as both
// a Vietnamese modifier and a literal character.
func (e *Engine) handleAppendingKey(keyRune rune, oldText string) (string, bool) {
	fullSeq := e.preeditor.GetProcessedString(bamboo.VietnameseMode)
	var newText string
	if e.shouldFallbackToEnglish(true) {
		newText = e.getProcessedString(bamboo.EnglishMode)
	} else {
		newText = e.getProcessedString(bamboo.VietnameseMode)
	}

	if len(fullSeq) > 0 && rune(fullSeq[len(fullSeq)-1]) == keyRune {
		// [[ => [  (second press of appending key emits literal character)
		ret := e.getPreeditString()
		lastRune := rune(ret[len(ret)-1])
		isWBS := bamboo.IsWordBreakSymbol(lastRune)
		if isWBS {
			e.preeditor.RemoveLastChar(false)
			e.preeditor.ProcessKey(' ', bamboo.EnglishMode)
		}
		return ret, isWBS
	}
	if l := []rune(newText); len(l) > 0 && keyRune == l[len(l)-1] {
		// f] => f]
		isWBS := bamboo.IsWordBreakSymbol(keyRune)
		if isWBS {
			e.preeditor.RemoveLastChar(false)
			e.preeditor.ProcessKey(' ', bamboo.EnglishMode)
		}
		return oldText + string(keyRune), isWBS
	}
	// ] => ơ (appending key consumed as Vietnamese modifier)
	return e.getPreeditString(), false
}

func (e *Engine) handleNonVnWord(keyVal, keyCode, state uint32) string {
	isPrintable := e.isPrintableKey(state, keyVal)
	keyRune := rune(keyVal)
	oldText := e.getPreeditString()

	var keyS string
	if isPrintable {
		keyS = string(keyRune)
	}

	if bamboo.HasAnyVietnameseRune(oldText) && e.mustFallbackToEnglish() {
		e.preeditor.RestoreLastWord(false)
		newText := e.preeditor.GetProcessedString(bamboo.PunctuationMode|bamboo.EnglishMode) + keyS
		if isPrintable {
			e.preeditor.ProcessKey(keyRune, bamboo.EnglishMode)
		}
		return newText
	}

	if isPrintable {
		e.preeditor.ProcessKey(keyRune, bamboo.EnglishMode)
		return oldText + keyS
	}
	// Non-printable key (e.g. Ctrl+A) is treated as a word-break symbol
	return oldText + keyS
}
