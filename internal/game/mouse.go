package game

import (
	"math/rand"
	"time"
	"unsafe"

	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/hectorgimenez/koolo/internal/utils/winproc"
	"github.com/lxn/win"
)

const (
	RightButton MouseButton = win.MK_RBUTTON
	LeftButton  MouseButton = win.MK_LBUTTON

	ShiftKey ModifierKey = win.VK_SHIFT
	CtrlKey  ModifierKey = win.VK_CONTROL
)

type MouseButton uint
type ModifierKey byte

const pointerReleaseDelay = 150 * time.Millisecond

// MovePointer moves the mouse to the requested position, x and y should be the final position based on
// pixels shown in the screen. Top-left corner is 0,0
func (hid *HID) MovePointer(x, y int) {
	hid.gr.updateWindowPositionData()
	x, y = hid.scaleToWindow(x, y)
	sx := hid.gr.WindowLeftX + x
	sy := hid.gr.WindowTopY + y

	// Ensure the game window has focus so input reaches it.
	if hid.gr.HWND != 0 {
		winproc.SetForegroundWindow.Call(uintptr(hid.gr.HWND))
		utils.Sleep(20)
	}

	winproc.SetCursorPos.Call(uintptr(sx), uintptr(sy))

	// SendInput absolute move so the game sees real mouse-move events.
	// SetCursorPos alone (and PostMessage WM_MOUSEMOVE) is not enough for
	// D2R's DirectInput-based hover detection - the NPC hover flag never
	// got set under Wine.
	const (
		MOUSEEVENTF_ABSOLUTE    = 0x8000
		MOUSEEVENTF_VIRTUALDESK = 0x4000
	)
	vx := int(win.GetSystemMetrics(win.SM_XVIRTUALSCREEN))
	vy := int(win.GetSystemMetrics(win.SM_YVIRTUALSCREEN))
	vw := int(win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN))
	vh := int(win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN))
	if vw > 0 && vh > 0 {
		absX := uintptr((sx - vx) * 65535 / (vw - 1))
		absY := uintptr((sy - vy) * 65535 / (vh - 1))
		sendInputMouse(absX, absY, 0, MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK)
	}
}

// scaleToWindow converts game virtual coordinates (1280x720) to actual
// window client pixels. Wine/Proton may scale the window size.
func (hid *HID) scaleToWindow(x, y int) (int, int) {
	gx, gy := hid.gr.GameAreaSizeX, hid.gr.GameAreaSizeY
	if gx > 0 && gy > 0 && (gx != 1280 || gy != 720) {
		x = x * gx / 1280
		y = y * gy / 720
	}
	return x, y
}

// Click just does a single mouse click at current pointer position.
// Uses SetCursorPos + SendInput (hardware-level input) which works
// more reliably under Wine/Proton than SendMessage(WM_LBUTTONDOWN).
func (hid *HID) Click(btn MouseButton, x, y int) {
	hid.gr.updateWindowPositionData()
	x, y = hid.scaleToWindow(x, y)
	sx := hid.gr.WindowLeftX + x
	sy := hid.gr.WindowTopY + y

	// Ensure the game window has focus so SendInput reaches it.
	if hid.gr.HWND != 0 {
		r, _, _ := winproc.SetForegroundWindow.Call(uintptr(hid.gr.HWND))
		if r == 0 {
			// SetForegroundWindow failed; try SetWindowPos to bring to front first.
			const swpNoMove = 0x0002
			const swpNoSize = 0x0001
			const swpShowWindow = 0x0040
			win.SetWindowPos(hid.gr.HWND, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
			utils.Sleep(20)
			winproc.SetForegroundWindow.Call(uintptr(hid.gr.HWND))
		}
		utils.Sleep(20)
	}

	// Move cursor to target position.
	winproc.SetCursorPos.Call(uintptr(sx), uintptr(sy))
	utils.Sleep(50)

	// SendInput MOUSEINPUT for hardware-level click (works with DirectInput under Wine).
	const (
		MOUSEEVENTF_LEFTDOWN    = 0x0002
		MOUSEEVENTF_LEFTUP      = 0x0004
		MOUSEEVENTF_RIGHTDOWN   = 0x0008
		MOUSEEVENTF_RIGHTUP     = 0x0010
		MOUSEEVENTF_ABSOLUTE    = 0x8000
		MOUSEEVENTF_VIRTUALDESK = 0x4000
	)
	// Absolute coords: normalize to 0..65535 across virtual screen.
	vx := int(win.GetSystemMetrics(win.SM_XVIRTUALSCREEN))
	vy := int(win.GetSystemMetrics(win.SM_YVIRTUALSCREEN))
	vw := int(win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN))
	vh := int(win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN))
	if vw > 0 && vh > 0 {
		absX := uintptr((sx - vx) * 65535 / (vw - 1))
		absY := uintptr((sy - vy) * 65535 / (vh - 1))
		downFlag := uintptr(MOUSEEVENTF_LEFTDOWN)
		upFlag := uintptr(MOUSEEVENTF_LEFTUP)
		if btn == RightButton {
			downFlag = MOUSEEVENTF_RIGHTDOWN
			upFlag = MOUSEEVENTF_RIGHTUP
		}
		// SendInput with absolute coordinates (type=0 MOUSEINPUT, cbSize=40).
		// MOUSEINPUT struct: dx(4) dy(4) mouseData(4) dwFlags(4) time(4) dwExtraInfo(8)
		sendInputMouse(absX, absY, 0, MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK)
		utils.Sleep(30)
		sendInputMouse(absX, absY, 0, downFlag)
		// Also send mouse_event as fallback (some Wine builds only honor one).
		winproc.MouseEvent.Call(downFlag, 0, 0, 0, 0)
		sleepTime := rand.Intn(keyPressMaxTime-keyPressMinTime) + keyPressMinTime
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		sendInputMouse(absX, absY, 0, upFlag)
		winproc.MouseEvent.Call(upFlag, 0, 0, 0, 0)
		return
	}

	// Fallback: mouse_event.
	downFlag := uintptr(MOUSEEVENTF_LEFTDOWN)
	upFlag := uintptr(MOUSEEVENTF_LEFTUP)
	if btn == RightButton {
		downFlag = MOUSEEVENTF_RIGHTDOWN
		upFlag = MOUSEEVENTF_RIGHTUP
	}
	winproc.MouseEvent.Call(downFlag, 0, 0, 0, 0)
	sleepTime := rand.Intn(keyPressMaxTime-keyPressMinTime) + keyPressMinTime
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	winproc.MouseEvent.Call(upFlag, 0, 0, 0, 0)
}

func (hid *HID) ClickWithModifier(btn MouseButton, x, y int, modifier ModifierKey) {
	hid.gi.OverrideGetKeyState(byte(modifier))
	hid.Click(btn, x, y)
	hid.gi.RestoreGetKeyState()
}

func calculateLparam(x, y int) uintptr {
	return uintptr(y<<16 | x)
}

// sendInputMouse sends a single mouse input event via SendInput.
// dx/dy are absolute virtual-screen coordinates (0..65535 when MOUSEEVENTF_ABSOLUTE is set).
func sendInputMouse(dx, dy, data, flags uintptr) {
	// INPUT struct: type(4) + pad(4) + MOUSEINPUT(dx(4) dy(4) mouseData(4) dwFlags(4) time(4) pad(4) dwExtraInfo(8)) = 40 bytes on x64
	const INPUT_MOUSE = 0
	const INPUT_SIZE = 40
	buf := make([]byte, INPUT_SIZE)
	// type = INPUT_MOUSE
	*(*uint32)(unsafe.Pointer(&buf[0])) = INPUT_MOUSE
	// MOUSEINPUT at offset 8
	*(*int32)(unsafe.Pointer(&buf[8])) = int32(dx)
	*(*int32)(unsafe.Pointer(&buf[12])) = int32(dy)
	*(*uint32)(unsafe.Pointer(&buf[16])) = uint32(data)
	*(*uint32)(unsafe.Pointer(&buf[20])) = uint32(flags)
	// time and dwExtraInfo stay 0
	winproc.SendInput.Call(1, uintptr(unsafe.Pointer(&buf[0])), INPUT_SIZE)
}
