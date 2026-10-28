package game

import (
	"math/rand"
	"strings"
	"time"
	"unsafe"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/koolo/internal/utils/winproc"
	"github.com/lxn/win"
)

const (
	keyPressMinTime = 40 // ms
	keyPressMaxTime = 90 // ms
)

// PressKey receives an ASCII code and sends a key press event to the game window.
// Uses SendInput (hardware-level), same as Click(): D2R reads keys via
// DirectInput and ignores PostMessage(WM_KEYDOWN), which made ESC never
// open the quit menu and game exit time out under Wine.
func (hid *HID) PressKey(key byte) {
	sendInputKey(key, false)
	sleepTime := rand.Intn(keyPressMaxTime-keyPressMinTime) + keyPressMinTime
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	sendInputKey(key, true)
}

func (hid *HID) KeySequence(keysToPress ...byte) {
	for _, key := range keysToPress {
		hid.PressKey(key)
		time.Sleep(200 * time.Millisecond)
	}
}

// PressKeyWithModifier works the same as PressKey but with a modifier key (shift, ctrl, alt)
func (hid *HID) PressKeyWithModifier(key byte, modifier ModifierKey) {
	hid.gi.OverrideGetKeyState(byte(modifier))
	hid.PressKey(key)
	hid.gi.RestoreGetKeyState()
}

func (hid *HID) PressKeyBinding(kb data.KeyBinding) {
	keys := getKeysForKB(kb)
	if keys[1] == 0 || keys[1] == 255 {
		hid.PressKey(keys[0])
		return
	}

	hid.PressKeyWithModifier(keys[0], ModifierKey(keys[1]))
}

// KeyDown sends a key down event to the game window
func (hid *HID) KeyDown(kb data.KeyBinding) {
	keys := getKeysForKB(kb)
	sendInputKey(keys[0], false)
}

// KeyUp sends a key up event to the game window
func (hid *HID) KeyUp(kb data.KeyBinding) {
	keys := getKeysForKB(kb)
	sendInputKey(keys[0], true)
}

// sendInputKey sends a single keyboard input event via SendInput (hardware-level).
func sendInputKey(key byte, up bool) {
	const INPUT_KEYBOARD = 1
	const KEYEVENTF_KEYUP = 0x0002
	const INPUT_SIZE = 40
	if key == 0 || key == 255 {
		return
	}
	buf := make([]byte, INPUT_SIZE)
	// type = INPUT_KEYBOARD
	*(*uint32)(unsafe.Pointer(&buf[0])) = INPUT_KEYBOARD
	// KEYBDINPUT at offset 8: wVk(2) wScan(2) dwFlags(4) time(4) pad(4) dwExtraInfo(8)
	*(*uint16)(unsafe.Pointer(&buf[8])) = uint16(key)
	scan, _, _ := winproc.MapVirtualKey.Call(uintptr(key), 0)
	*(*uint16)(unsafe.Pointer(&buf[10])) = uint16(scan)
	flags := uint32(0)
	if up {
		flags = KEYEVENTF_KEYUP
	}
	*(*uint32)(unsafe.Pointer(&buf[12])) = flags
	// time and dwExtraInfo stay 0
	winproc.SendInput.Call(1, uintptr(unsafe.Pointer(&buf[0])), INPUT_SIZE)
}

func getKeysForKB(kb data.KeyBinding) [2]byte {
	if kb.Key1[0] == 0 || kb.Key1[0] == 255 {
		return [2]byte{kb.Key2[0], kb.Key2[1]}
	}

	return [2]byte{kb.Key1[0], kb.Key1[1]}
}

func (hid *HID) GetASCIICode(key string) byte {
	char, found := specialChars[strings.ToLower(key)]
	if found {
		return char
	}

	return strings.ToUpper(key)[0]
}

var specialChars = map[string]byte{
	"esc":       win.VK_ESCAPE,
	"enter":     win.VK_RETURN,
	"f1":        win.VK_F1,
	"f2":        win.VK_F2,
	"f3":        win.VK_F3,
	"f4":        win.VK_F4,
	"f5":        win.VK_F5,
	"f6":        win.VK_F6,
	"f7":        win.VK_F7,
	"f8":        win.VK_F8,
	"f9":        win.VK_F9,
	"f10":       win.VK_F10,
	"f11":       win.VK_F11,
	"f12":       win.VK_F12,
	"lctrl":     win.VK_LCONTROL,
	"home":      win.VK_HOME,
	"down":      win.VK_DOWN,
	"up":        win.VK_UP,
	"left":      win.VK_LEFT,
	"right":     win.VK_RIGHT,
	"tab":       win.VK_TAB,
	"space":     win.VK_SPACE,
	"alt":       win.VK_MENU,
	"lalt":      win.VK_LMENU,
	"ralt":      win.VK_RMENU,
	"shift":     win.VK_LSHIFT,
	"backspace": win.VK_BACK,
	"lwin":      win.VK_LWIN,
	"rwin":      win.VK_RWIN,
	"end":       win.VK_END,
	"-":         win.VK_OEM_MINUS,
}

func (hid *HID) calculatelParam(keyCode byte, down bool) uintptr {
	ret, _, _ := winproc.MapVirtualKey.Call(uintptr(keyCode), 0)
	scanCode := int(ret)
	repeatCount := 1
	extendedKeyFlag := 0
	contextCode := 0
	previousKeyState := 0
	transitionState := 0
	if !down {
		transitionState = 1
	}

	lParam := uintptr((repeatCount & 0xFFFF) | (scanCode << 16) | (extendedKeyFlag << 24) | (contextCode << 29) | (previousKeyState << 30) | (transitionState << 31))
	return lParam
}
