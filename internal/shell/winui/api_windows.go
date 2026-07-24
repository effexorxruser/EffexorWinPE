//go:build windows

package winui

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modGdi32    = windows.NewLazySystemDLL("gdi32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modComctl32 = windows.NewLazySystemDLL("comctl32.dll")
	modShell32  = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClassExW              = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW               = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW                = modUser32.NewProc("DefWindowProcW")
	procShowWindow                    = modUser32.NewProc("ShowWindow")
	procUpdateWindow                  = modUser32.NewProc("UpdateWindow")
	procGetMessageW                   = modUser32.NewProc("GetMessageW")
	procTranslateMessage              = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW              = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage               = modUser32.NewProc("PostQuitMessage")
	procDestroyWindow                 = modUser32.NewProc("DestroyWindow")
	procLoadCursorW                   = modUser32.NewProc("LoadCursorW")
	procSetWindowTextW                = modUser32.NewProc("SetWindowTextW")
	procGetClientRect                 = modUser32.NewProc("GetClientRect")
	procMoveWindow                    = modUser32.NewProc("MoveWindow")
	procSendMessageW                  = modUser32.NewProc("SendMessageW")
	procGetDlgCtrlID                  = modUser32.NewProc("GetDlgCtrlID")
	procMessageBoxW                   = modUser32.NewProc("MessageBoxW")
	procEnableWindow                  = modUser32.NewProc("EnableWindow")
	procSetFocus                      = modUser32.NewProc("SetFocus")
	procGetSystemMetrics              = modUser32.NewProc("GetSystemMetrics")
	procFillRect                      = modUser32.NewProc("FillRect")
	procBeginPaint                    = modUser32.NewProc("BeginPaint")
	procEndPaint                      = modUser32.NewProc("EndPaint")
	procInvalidateRect                = modUser32.NewProc("InvalidateRect")
	procGetDC                         = modUser32.NewProc("GetDC")
	procReleaseDC                     = modUser32.NewProc("ReleaseDC")
	procSetProcessDPIAware            = modUser32.NewProc("SetProcessDPIAware")
	procCreateSolidBrush              = modGdi32.NewProc("CreateSolidBrush")
	procSetBkColor                    = modGdi32.NewProc("SetBkColor")
	procSetTextColor                  = modGdi32.NewProc("SetTextColor")
	procDeleteObject                  = modGdi32.NewProc("DeleteObject")
	procCreateFontW                   = modGdi32.NewProc("CreateFontW")
	procSelectObject                  = modGdi32.NewProc("SelectObject")
	procGetModuleHandleW              = modKernel32.NewProc("GetModuleHandleW")
	procInitCommonControlsEx          = modComctl32.NewProc("InitCommonControlsEx")
	procSHBrowseForFolderW            = modShell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW          = modShell32.NewProc("SHGetPathFromIDListW")
	procExitWindowsEx                 = modUser32.NewProc("ExitWindowsEx")
	procPostMessageW                  = modUser32.NewProc("PostMessageW")
	procSetWindowPos                  = modUser32.NewProc("SetWindowPos")
	procGetWindowLongPtrW             = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW             = modUser32.NewProc("SetWindowLongPtrW")
	procGetLogicalDriveStringsW       = modKernel32.NewProc("GetLogicalDriveStringsW")
	procGetDriveTypeW                 = modKernel32.NewProc("GetDriveTypeW")
	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsTabstop          = 0x00010000
	wsBorder           = 0x00800000
	wsVScroll          = 0x00200000
	wsHScroll          = 0x00100000
	esMultiline        = 0x0004
	esReadonly         = 0x0800
	esAutovscroll      = 0x0040
	esWantreturn       = 0x1000
	lbsNotify          = 0x0001
	bsPushbutton       = 0x00000000
	swShow             = 5
	cwUseDefault       = 0x80000000

	wmCreate          = 0x0001
	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmCommand         = 0x0111
	wmPaint           = 0x000F
	wmEraseBkgnd      = 0x0014
	wmSetfont         = 0x0030
	wmCtlColorEdit    = 0x0133
	wmCtlColorListBox = 0x0134
	wmCtlColorBtn     = 0x0135
	wmCtlColorStatic  = 0x0138
	wmDpiChanged      = 0x02E0
	wmApp             = 0x8000
	wmKeyDown         = 0x0100

	vkEscape = 0x1B

	lbnSelchange = 1
	bnClicked    = 0

	mbOK       = 0x00000000
	mbOKCancel = 0x00000001
	mbYesNo    = 0x00000004
	mbIconWarn = 0x00000030
	idOK       = 1
	idYes      = 6

	smCXscreen = 0
	smCYscreen = 1

	ewxShutdown = 0x00000001
	ewxReboot   = 0x00000002
	ewxForce    = 0x00000004

	colorWindow   = 5
	idfBrowseInfo = 0x00000040 // BIF_NEWDIALOGSTYLE

	gwlStyle      = -16
	wsCaption     = 0x00C00000
	wsThickFrame  = 0x00040000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000

	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
	swMaximize      = 3
	swRestore       = 9

	driveRemovable = 2
	driveFixed     = 3

	msgUIRefresh  = wmApp + 1
	msgUIProgress = wmApp + 2
	msgUIDone     = wmApp + 3
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	HDC         windows.Handle
	Erase       int32
	PaintRect   rect
	Restore     int32
	IncUpdate   int32
	RGBReserved [32]byte
}

type msg struct {
	HWND    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type point struct{ X, Y int32 }

type initCommonControlsEx struct {
	Size uint32
	ICC  uint32
}

type browseInfo struct {
	Owner       windows.HWND
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	LParam      uintptr
	Image       int32
}

func utf16ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}

func rgb(r, g, b uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func loWord(v uintptr) uint16 { return uint16(v & 0xFFFF) }
func hiWord(v uintptr) uint16 { return uint16((v >> 16) & 0xFFFF) }

func getModuleHandle() windows.Handle {
	h, _, _ := procGetModuleHandleW.Call(0)
	return windows.Handle(h)
}

func registerClass(cls *wndClassEx) (uint16, error) {
	r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(cls)))
	if r == 0 {
		return 0, err
	}
	return uint16(r), nil
}

func createWindow(exStyle uint32, className, windowName string, style uint32, x, y, w, h int32, parent windows.HWND, menu uintptr, instance windows.Handle, param uintptr) (windows.HWND, error) {
	r, _, err := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(utf16ptr(className))),
		uintptr(unsafe.Pointer(utf16ptr(windowName))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent),
		menu,
		uintptr(instance),
		param,
	)
	if r == 0 {
		return 0, err
	}
	return windows.HWND(r), nil
}

func defWindowProc(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func sendMessage(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func setWindowText(hwnd windows.HWND, text string) {
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(utf16ptr(text))))
}

func messageBox(owner windows.HWND, text, caption string, flags uint32) int {
	r, _, _ := procMessageBoxW.Call(
		uintptr(owner),
		uintptr(unsafe.Pointer(utf16ptr(text))),
		uintptr(unsafe.Pointer(utf16ptr(caption))),
		uintptr(flags),
	)
	return int(r)
}

func createFont(height int32, face string) windows.Handle {
	h, _, _ := procCreateFontW.Call(
		uintptr(height), 0, 0, 0,
		uintptr(400), // FW_NORMAL
		0, 0, 0,
		uintptr(1), // DEFAULT_CHARSET
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16ptr(face))),
	)
	return windows.Handle(h)
}

func postMessage(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) {
	procPostMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
}

func setWindowPos(hwnd windows.HWND, x, y, cx, cy int32, flags uint32) {
	procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(flags))
}

func getWindowLongPtr(hwnd windows.HWND, index int32) uintptr {
	r, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return r
}

func setWindowLongPtr(hwnd windows.HWND, index int32, value uintptr) uintptr {
	r, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), value)
	return r
}

func logicalDrives() []string {
	n, _, _ := procGetLogicalDriveStringsW.Call(0, 0)
	if n == 0 {
		return nil
	}
	buf := make([]uint16, n)
	procGetLogicalDriveStringsW.Call(uintptr(n), uintptr(unsafe.Pointer(&buf[0])))
	var out []string
	start := 0
	for i, v := range buf {
		if v == 0 {
			if i > start {
				out = append(out, windows.UTF16ToString(buf[start:i]))
			}
			start = i + 1
		}
	}
	return out
}

func driveType(root string) uint32 {
	r, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(utf16ptr(root))))
	return uint32(r)
}

func browseForFolder(owner windows.HWND, title string) string {
	if procSHBrowseForFolderW.Find() != nil || procSHGetPathFromIDListW.Find() != nil {
		return ""
	}
	buf := make([]uint16, 260)
	bi := browseInfo{
		Owner:       owner,
		DisplayName: &buf[0],
		Title:       utf16ptr(title),
		Flags:       idfBrowseInfo,
	}
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	pathBuf := make([]uint16, 260)
	ok, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if ok == 0 {
		return ""
	}
	return windows.UTF16ToString(pathBuf)
}

// Keep syscall import for potential uintptr casts on older patterns.
var _ = syscall.EINVAL
