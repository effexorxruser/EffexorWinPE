//go:build windows

package winui

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	msgLockSmokePing = wmApp + 40
	msgLockSmokeFail = wmApp + 41
)

// TestLockedOSThreadReceivesPostedMessage verifies the Win32 thread-affinity
// contract used by Run: create the window and pump GetMessageW on a locked OS
// thread, while a background goroutine may only PostMessage.
func TestLockedOSThreadReceivesPostedMessage(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		// Win32 windows and their message queues are thread-affine; the creating
		// OS thread must also own the message loop.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- runLockedThreadPostMessageSmoke()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("locked OS thread message-loop smoke timed out")
	}
}

func runLockedThreadPostMessageSmoke() error {
	class := fmt.Sprintf("EffexorWinPELockOSThreadSmoke-%d", time.Now().UnixNano())
	var gotPing atomic.Bool

	wndProc := func(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case msgLockSmokePing:
			gotPing.Store(true)
			procPostQuitMessage.Call(0)
			return 0
		case msgLockSmokeFail:
			procPostQuitMessage.Call(1)
			return 0
		case wmDestroy:
			return 0
		}
		return defWindowProc(hwnd, msg, wParam, lParam)
	}
	// Keep the callback alive for the lifetime of the window/class.
	cb := windows.NewCallback(wndProc)

	inst := getModuleHandle()
	cls := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   cb,
		Instance:  inst,
		Cursor:    mustLoadCursor(),
		ClassName: utf16ptr(class),
	}
	if _, err := registerClass(&cls); err != nil {
		// Class may already exist from a previous failed run in this process.
		_ = err
	}

	hwnd, err := createWindow(0, class, "lock-os-thread-smoke", wsOverlappedWindow, 0, 0, 8, 8, 0, 0, inst, 0)
	if err != nil {
		return fmt.Errorf("create hidden window: %w", err)
	}
	procShowWindow.Call(uintptr(hwnd), 0) // SW_HIDE

	go func() {
		time.Sleep(20 * time.Millisecond)
		postMessage(hwnd, msgLockSmokePing, 0, 0)
	}()

	// Failsafe: never hang the test if PostMessage/GetMessage misbehave.
	time.AfterFunc(3*time.Second, func() {
		postMessage(hwnd, msgLockSmokeFail, 0, 0)
	})

	var m msg
	for {
		ret, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			procDestroyWindow.Call(uintptr(hwnd))
			return fmt.Errorf("GetMessageW failed: %w", callErr)
		case 0:
			procDestroyWindow.Call(uintptr(hwnd))
			if !gotPing.Load() {
				return fmt.Errorf("WM_QUIT without receiving posted ping (wParam=%d)", m.WParam)
			}
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
