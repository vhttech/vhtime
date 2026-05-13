package main

import (
	"strconv"
	"strings"

	"vhtime/config"

	ibus "vhtime/goibus"
)

func (e *Engine) getInputMode() int {
	wmClass := e.getWmClass()
	if wmClass != "" {
		if im, ok := e.config.InputModeMapping[wmClass]; ok && config.ImLookupTable[im] != "" {
			return im
		}
		// GPU terminals on X11 don't support XIM preedit or IBus CommitText;
		// ForwardAsCommitIM delivers each character as an XIM key event which Kitty
		// and similar terminals do handle correctly.
		if !isWayland && inStringList(x11NonXimTerminals, wmClass) {
			return config.ForwardAsCommitIM
		}
		// Browsers must NOT use PreeditIM. PreeditIM puts the browser into IME
		// composition mode which disables the address-bar inline autocomplete
		// (the grey "cebook.com" suffix). Without inline autocomplete, pressing
		// Enter searches instead of navigating to the top suggestion.
		// BackspaceForwardingIM commits text directly without creating a preedit
		// composition, so the address bar behaves identically to native typing.
		// Vietnamese correction still works via CommitText + backspace pairs.
		// Users can override per-app via the input mode picker.
		if inStringList(DefaultBrowserList, wmClass) {
			return config.BackspaceForwardingIM
		}
	}
	if _, ok := config.ImLookupTable[e.config.DefaultInputMode]; ok {
		return e.config.DefaultInputMode
	}
	return config.PreeditIM
}

func (e *Engine) checkInputMode(im int) bool {
	return e.getInputMode() == im
}

func (e *Engine) inBackspaceWhiteList() bool {
	inputMode := e.getInputMode()
	for _, im := range config.ImBackspaceList {
		if im == inputMode {
			return true
		}
	}
	return false
}

// openLookupTable shows the input mode picker for the current window class.
func (e *Engine) openLookupTable() {
	wmClass := e.getWmClass()
	displayClass := wmClass
	if parts := strings.Split(wmClass, ":"); len(parts) == 2 {
		displayClass = parts[1]
	}

	e.UpdateAuxiliaryText(ibus.NewText("Nhấn (1/2/3/4/5/6/7) để lưu tùy chọn của bạn"), true)

	lt := ibus.NewLookupTable()
	lt.PageSize = uint32(len(config.ImLookupTable))
	lt.Orientation = IBusOrientationVertical
	for im := 1; im <= len(config.ImLookupTable); im++ {
		if e.getInputMode() == im {
			lt.AppendLabel("*")
			lt.SetCursorPos(uint32(im - 1))
		} else {
			lt.AppendLabel(strconv.Itoa(im))
		}
		if im == config.UsIM {
			lt.AppendCandidate(config.ImLookupTable[im] + " (" + displayClass + ")")
		} else {
			lt.AppendCandidate(config.ImLookupTable[im])
		}
	}
	e.inputModeLookupTable = lt
	e.UpdateLookupTable(lt, true)
}

func (e *Engine) modeSwitcherKeyEvent(keyVal uint32, keyCode uint32, state uint32) (bool, bool) {
	if e.getWmClass() == "" {
		return true, true
	}
	if e.isShortcutKeyPressed(keyVal, state, KSInputModeSwitch) {
		e.closeInputModeCandidates()
		return true, false
	}
	var keyRune = rune(keyVal)
	switch {
	case keyVal == IBusLeft || keyVal == IBusUp:
		e.CursorUp()
		return true, true
	case keyVal == IBusRight || keyVal == IBusDown:
		e.CursorDown()
		return true, true
	case keyVal == IBusPageUp:
		e.PageUp()
		return true, true
	case keyVal == IBusPageDown:
		e.PageDown()
		return true, true
	case keyVal == IBusReturn:
		e.commitInputModeCandidate()
		e.closeInputModeCandidates()
		return true, true
	case keyVal == IBusEscape:
		e.closeInputModeCandidates()
		return true, false
	}
	if keyRune >= '1' && keyRune <= '7' {
		if pos, err := strconv.Atoi(string(keyRune)); err == nil {
			if e.inputModeLookupTable.SetCursorPos(uint32(pos - 1)) {
				e.commitInputModeCandidate()
				e.closeInputModeCandidates()
				return true, true
			}
			e.closeInputModeCandidates()
		}
	}
	return true, false
}

func (e *Engine) commitInputModeCandidate() {
	im := int(e.inputModeLookupTable.CursorPos + 1)
	e.config.InputModeMapping[e.getWmClass()] = im
	config.SaveConfig(e.config, e.engineName)
	e.propList = GetPropListByConfig(e.config)
	e.RegisterProperties(e.propList)
}

func (e *Engine) closeInputModeCandidates() {
	e.inputModeLookupTable = nil
	e.UpdateLookupTable(ibus.NewLookupTable(), true) // workaround for issue #18
	e.HidePreeditText()
	e.HideLookupTable()
	e.HideAuxiliaryText()
	e.isInputModeLTOpened = false
}

func (e *Engine) updateInputModeLT() {
	visible := len(e.inputModeLookupTable.Candidates) > 0
	e.UpdateLookupTable(e.inputModeLookupTable, visible)
}
