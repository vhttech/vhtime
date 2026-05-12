package main

import (
	ibus "vhtime/goibus"
	"github.com/godbus/dbus/v5"
)

// fakeEngine is a test double for IEngine that records emitted signals so that
// test assertions can inspect what the engine sent to the client.
type fakeEngine struct {
	commitText          string
	preeditText         string
	committed           bool
	isHidePreeditText   bool
	isHideAuxiliaryText bool
	isHideLookupTable   bool
	isReset             bool
	forwardKeyEvent     [3]uint32
	// forwardedText accumulates characters delivered via ForwardKeyEvent press events.
	// This mirrors what a terminal (e.g. Kitty) would receive when ForwardAsCommitIM is active.
	forwardedText string
}

// reverseVnSymMapping converts X11 keysyms back to the Go rune they represent.
// Some legacy keysyms (e.g. 0x01F0 for 'đ') don't match their Unicode codepoint,
// so we can't decode them by arithmetic alone.
var reverseVnSymMapping = func() map[uint32]rune {
	m := make(map[uint32]rune, len(vnSymMapping))
	for r, ks := range vnSymMapping {
		m[ks] = r
	}
	return m
}()

func NewFakeEngine() *fakeEngine {
	return &fakeEngine{}
}

func (e *fakeEngine) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	items := make(map[string]dbus.Variant)
	return items, nil
}

func (e *fakeEngine) ProcessKeyEvent(keyval uint32, keycode uint32, state uint32) (bool, *dbus.Error) {
	return false, nil
}

func (e *fakeEngine) SetCursorLocation(x int32, y int32, w int32, h int32) *dbus.Error {
	return nil
}

func (e *fakeEngine) SetSurroundingText(text dbus.Variant, cursor_index uint32, anchor_pos uint32) *dbus.Error {
	return nil
}

func (e *fakeEngine) SetCapabilities(cap uint32) *dbus.Error {
	return nil
}

func (e *fakeEngine) FocusIn() *dbus.Error {
	return nil
}

func (e *fakeEngine) FocusOut() *dbus.Error {
	return nil
}

func (e *fakeEngine) Reset() *dbus.Error {
	e.isReset = true
	return nil
}

// @method()
func (e *fakeEngine) PageUp() *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) PageDown() *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) CursorUp() *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) CursorDown() *dbus.Error {
	return nil
}

// @method(in_signature="uuu")
func (e *fakeEngine) CandidateClicked(index uint32, button uint32, state uint32) *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) Enable() *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) Disable() *dbus.Error {
	return nil
}

// @method(in_signature="su")
func (e *fakeEngine) PropertyActivate(prop_name string, prop_state uint32) *dbus.Error {
	return nil
}

// @method(in_signature="s")
func (e *fakeEngine) PropertyShow(prop_name string) *dbus.Error {
	return nil
}

// @method(in_signature="s")
func (e *fakeEngine) PropertyHide(prop_name string) *dbus.Error {
	return nil
}

// @method()
func (e *fakeEngine) Destroy() *dbus.Error {
	return nil
}

// @signal(signature="v")
func (e *fakeEngine) CommitText(text *ibus.Text) {
	e.commitText += text.Text
}

// @signal(signature="uuu")
func (e *fakeEngine) ForwardKeyEvent(keyval uint32, keycode uint32, state uint32) {
	e.forwardKeyEvent = [3]uint32{keyval, keycode, state}
	if state&IBusReleaseMask != 0 {
		return
	}
	if keyval == IBusBackSpace {
		runes := []rune(e.forwardedText)
		if len(runes) > 0 {
			e.forwardedText = string(runes[:len(runes)-1])
		}
		return
	}
	var cp rune
	if r, ok := reverseVnSymMapping[keyval]; ok {
		cp = r
	} else if keyval >= 0x01000000 {
		cp = rune(keyval - 0x01000000)
	} else if keyval >= 0x20 && keyval < 0x10000 {
		cp = rune(keyval)
	}
	if cp != 0 {
		e.forwardedText += string(cp)
	}
}

// @signal(signature="vubu")
func (e *fakeEngine) UpdatePreeditText(text *ibus.Text, cursor_pos uint32, visible bool) {
	e.preeditText = text.Text
}
func (e *fakeEngine) UpdatePreeditTextWithMode(text *ibus.Text, cursor_pos uint32, visible bool, mode uint32) {
	e.preeditText = text.Text
}

// @signal()
func (e *fakeEngine) ShowPreeditText() {
}

// @signal()
func (e *fakeEngine) HidePreeditText() {
	e.preeditText = ""
	e.isHidePreeditText = true
}

// @signal(signature="vb")
func (e *fakeEngine) UpdateAuxiliaryText(text *ibus.Text, visible bool) {
}

// @signal()
func (e *fakeEngine) ShowAuxiliaryText() {
	e.isHideAuxiliaryText = false
}

// @signal()
func (e *fakeEngine) HideAuxiliaryText() {
	e.isHideAuxiliaryText = true
}

// @signal(signature="vb")
func (e *fakeEngine) UpdateLookupTable(lookup_table *ibus.LookupTable, visible bool) {
}

// @signal()
func (e *fakeEngine) ShowLookupTable() {
	e.isHideLookupTable = false
}

// @signal()
func (e *fakeEngine) HideLookupTable() {
	e.isHideLookupTable = true
}

// @signal()
func (e *fakeEngine) PageUpLookupTable() {
}

// @signal()
func (e *fakeEngine) PageDownLookupTable() {
}

// @signal()
func (e *fakeEngine) CursorUpLookupTable() {
}

// @signal()
func (e *fakeEngine) CursorDownLookupTable() {
}

// @signal(signature="v")
func (e *fakeEngine) RegisterProperties(props *ibus.PropList) {
}

// @signal(signature="v")
func (e *fakeEngine) UpdateProperty(prop *ibus.Property) {
}

// @signal(signature="iu")
func (e *fakeEngine) DeleteSurroundingText(offset_from_cursor int32, nchars uint32) {
	s := []rune(e.commitText)
	var txt string
	for _, ch := range s[:len(s)-int(nchars)] {
		txt += string(ch)
	}
	e.commitText = txt
}

// @signal()
func (e *fakeEngine) RequireSurroundingText() {
}
