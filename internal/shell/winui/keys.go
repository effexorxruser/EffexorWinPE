package winui

// Win32 message / virtual-key constants used for kiosk keyboard routing.
// Kept in a portable file so routing logic is unit-tested without Windows.
const (
	keyMsgKeyDown    = 0x0100 // WM_KEYDOWN
	keyMsgSysKeyDown = 0x0104 // WM_SYSKEYDOWN
	keyVKEscape      = 0x1B
)

// ShouldExitKioskOnKeyMessage reports whether a raw Win32 message should exit
// fullscreen/kiosk mode. Call this from the main message loop before
// DispatchMessage so Esc is handled even when focus is on a child control
// (LISTBOX/EDIT/BUTTON) that would otherwise swallow WM_KEYDOWN.
func ShouldExitKioskOnKeyMessage(message uint32, wParam uintptr, kioskActive bool) bool {
	if !kioskActive {
		return false
	}
	if message != keyMsgKeyDown && message != keyMsgSysKeyDown {
		return false
	}
	return uint32(wParam)&0xffff == keyVKEscape
}
