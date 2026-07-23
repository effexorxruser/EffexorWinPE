//go:build windows

package export

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetLogicalDriveStringsW = modKernel32.NewProc("GetLogicalDriveStringsW")
	procGetDriveTypeW           = modKernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformationW   = modKernel32.NewProc("GetVolumeInformationW")
	procGetDiskFreeSpaceExW     = modKernel32.NewProc("GetDiskFreeSpaceExW")
)

const (
	driveRemovable = 2
	driveFixed     = 3
)

// OSDriveScanner lists volumes using Win32 APIs.
type OSDriveScanner struct{}

func (OSDriveScanner) List() ([]DriveInfo, error) {
	n, _, err := procGetLogicalDriveStringsW.Call(0, 0)
	if n == 0 {
		return nil, err
	}
	buf := make([]uint16, n)
	procGetLogicalDriveStringsW.Call(n, uintptr(unsafe.Pointer(&buf[0])))
	var out []DriveInfo
	start := 0
	for i, v := range buf {
		if v != 0 {
			continue
		}
		if i <= start {
			start = i + 1
			continue
		}
		root := windows.UTF16ToString(buf[start:i])
		start = i + 1
		info := DriveInfo{Root: root}
		if len(root) > 0 {
			info.Letter = string(root[0])
		}
		dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(utf16ptr(root))))
		switch uint32(dt) {
		case driveRemovable:
			info.Kind = DriveRemovable
		case driveFixed:
			info.Kind = DriveFixed
		default:
			info.Kind = DriveOther
		}
		labelBuf := make([]uint16, 64)
		procGetVolumeInformationW.Call(
			uintptr(unsafe.Pointer(utf16ptr(root))),
			uintptr(unsafe.Pointer(&labelBuf[0])),
			uintptr(len(labelBuf)),
			0, 0, 0, 0, 0,
		)
		info.Label = windows.UTF16ToString(labelBuf)
		var total uint64
		procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(utf16ptr(root))),
			0,
			uintptr(unsafe.Pointer(&total)),
			0,
		)
		info.SizeBytes = total
		out = append(out, info)
	}
	return out, nil
}

func utf16ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}
