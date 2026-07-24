//go:build windows

package winui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/export"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/layout"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/orchestrator"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/present"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/uiqueue"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

const (
	className = "EffexorWinPEShellMain"

	idNavList      = 1001
	idContent      = 1002
	idBtnPrimary   = 1003
	idBtnSecondary = 1004
	idBtnTertiary  = 1005
	idBrand        = 1006
)

var (
	colorBg    = rgb(32, 36, 40)
	colorPanel = rgb(45, 50, 56)
	colorText  = rgb(232, 234, 237)
	colorBtnBg = rgb(58, 64, 72)
)

type app struct {
	cfg        Config
	bundle     *i18n.Bundle
	model      viewmodel.AppModel
	mu         sync.Mutex
	hwnd       windows.HWND
	nav        windows.HWND
	content    windows.HWND
	btnPrimary windows.HWND
	btnSecond  windows.HWND
	btnThird   windows.HWND
	brand      windows.HWND
	font       windows.Handle
	fontLarge  windows.Handle
	brushBg    windows.Handle
	brushPanel windows.Handle
	brushBtn   windows.Handle
	screens    []string
	current    string
	dpi        int32
	running    bool
	kiosk      bool
	windowedX  int32
	windowedY  int32
	windowedW  int32
	windowedH  int32
	queue      *uiqueue.Queue
	visibleBtn int
}

// Run starts the native Win32 message loop.
func Run(cfg Config) error {
	// Win32 windows and their message queues are thread-affine; the creating
	// OS thread must also own the message loop.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if cfg.Bundle == nil {
		b, err := i18n.New(i18n.Default)
		if err != nil {
			return err
		}
		cfg.Bundle = b
	}
	enableDPIAwareness()
	icc := initCommonControlsEx{Size: uint32(unsafe.Sizeof(initCommonControlsEx{})), ICC: 0xFFFF}
	_, _, _ = procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	a := &app{
		cfg:     cfg,
		bundle:  cfg.Bundle,
		model:   cfg.Model,
		screens: present.NavItems(),
		current: present.ScreenOverview,
		dpi:     96,
		kiosk:   cfg.Kiosk,
		queue:   uiqueue.New(),
	}
	if a.model.MockMode || cfg.Mock {
		a.model.MockMode = true
	}

	instance := getModuleHandle()
	bg, _, _ := procCreateSolidBrush.Call(uintptr(colorBg))
	panel, _, _ := procCreateSolidBrush.Call(uintptr(colorPanel))
	btn, _, _ := procCreateSolidBrush.Call(uintptr(colorBtnBg))
	a.brushBg = windows.Handle(bg)
	a.brushPanel = windows.Handle(panel)
	a.brushBtn = windows.Handle(btn)
	a.recreateFonts()

	cls := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:    windows.NewCallback(a.wndProc),
		Instance:   instance,
		Cursor:     mustLoadCursor(),
		Background: windows.Handle(a.brushBg),
		ClassName:  utf16ptr(className),
	}
	_, _ = registerClass(&cls)

	screenW, _, _ := procGetSystemMetrics.Call(smCXscreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCYscreen)
	width := int32(1100)
	height := int32(720)
	if width > int32(screenW) {
		width = int32(screenW)
	}
	if height > int32(screenH) {
		height = int32(screenH)
	}
	// Ensure usable layout on 1024x768.
	if width < 1024 && int32(screenW) >= 1024 {
		width = 1024
	}
	if height < 768 && int32(screenH) >= 768 {
		height = 768
	}
	x := (int32(screenW) - width) / 2
	y := (int32(screenH) - height) / 2
	a.windowedX, a.windowedY, a.windowedW, a.windowedH = x, y, width, height

	hwnd, err := createWindow(0, className, a.bundle.T("app.title"), wsOverlappedWindow|wsVisible, x, y, width, height, 0, 0, instance, 0)
	if err != nil {
		return err
	}
	a.hwnd = hwnd
	if a.kiosk {
		a.applyKiosk(true)
	} else {
		procShowWindow.Call(uintptr(hwnd), swMaximize)
	}
	procUpdateWindow.Call(uintptr(hwnd))

	var m msg
	for {
		ret, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			return fmt.Errorf("GetMessageW failed: %w", callErr)
		case 0:
			// WM_QUIT
			return nil
		}
		// Intercept Esc before DispatchMessage so child controls with focus
		// (LISTBOX/EDIT/BUTTON) cannot swallow kiosk exit.
		if ShouldExitKioskOnKeyMessage(m.Message, m.WParam, a.kiosk) {
			a.applyKiosk(false)
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func enableDPIAwareness() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if err := procSetProcessDpiAwarenessContext.Find(); err == nil {
		procSetProcessDpiAwarenessContext.Call(uintptr(uncheckedDPIContext(-4)))
		return
	}
	_, _, _ = procSetProcessDPIAware.Call()
}

func uncheckedDPIContext(v int) unsafe.Pointer {
	return unsafe.Pointer(uintptr(v))
}

func mustLoadCursor() windows.Handle {
	h, _, _ := procLoadCursorW.Call(0, uintptr(32512))
	return windows.Handle(h)
}

func (a *app) recreateFonts() {
	scale := float64(a.dpi) / 96.0
	if scale < 1 {
		scale = 1
	}
	body := int32(-18 * scale)
	title := int32(-28 * scale)
	if a.font != 0 {
		procDeleteObject.Call(uintptr(a.font))
	}
	if a.fontLarge != 0 {
		procDeleteObject.Call(uintptr(a.fontLarge))
	}
	a.font = createFont(body, "Segoe UI")
	a.fontLarge = createFont(title, "Segoe UI Semibold")
}

func (a *app) applyFonts() {
	for _, h := range []windows.HWND{a.nav, a.content, a.btnPrimary, a.btnSecond, a.btnThird} {
		if h != 0 {
			sendMessage(h, wmSetfont, uintptr(a.font), 1)
		}
	}
	if a.brand != 0 {
		sendMessage(a.brand, wmSetfont, uintptr(a.fontLarge), 1)
	}
}

func (a *app) wndProc(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCreate:
		a.hwnd = hwnd
		a.createChildren(hwnd)
		a.populateNav()
		a.refreshContent()
		return 0
	case wmSize:
		a.layout()
		return 0
	case wmDpiChanged:
		a.dpi = int32(loWord(wParam))
		a.recreateFonts()
		a.applyFonts()
		rc := (*rect)(unsafe.Pointer(lParam))
		if rc != nil {
			setWindowPos(hwnd, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, swpNoZOrder|swpShowWindow)
		}
		a.layout()
		a.refreshContent()
		return 0
	case wmKeyDown:
		if loWord(wParam) == vkEscape && a.kiosk {
			a.applyKiosk(false)
			return 0
		}
	case wmEraseBkgnd:
		var rc rect
		procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		procFillRect.Call(wParam, uintptr(unsafe.Pointer(&rc)), uintptr(a.brushBg))
		return 1
	case wmCtlColorEdit, wmCtlColorListBox, wmCtlColorStatic:
		procSetBkColor.Call(wParam, uintptr(colorPanel))
		procSetTextColor.Call(wParam, uintptr(colorText))
		return uintptr(a.brushPanel)
	case wmCtlColorBtn:
		procSetBkColor.Call(wParam, uintptr(colorBtnBg))
		procSetTextColor.Call(wParam, uintptr(colorText))
		return uintptr(a.brushBtn)
	case wmCommand:
		a.onCommand(wParam)
		return 0
	case msgUIProgress, msgUIDone, msgUIRefresh:
		a.drainUIEvents()
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
}

func (a *app) createChildren(parent windows.HWND) {
	inst := getModuleHandle()
	a.brand, _ = createWindow(0, "STATIC", a.bundle.T("app.brand"), wsChild|wsVisible, 0, 0, 0, 0, parent, uintptr(idBrand), inst, 0)
	a.nav, _ = createWindow(0, "LISTBOX", "", wsChild|wsVisible|wsBorder|wsVScroll|lbsNotify|wsTabstop, 0, 0, 0, 0, parent, uintptr(idNavList), inst, 0)
	a.content, _ = createWindow(0, "EDIT", "", wsChild|wsVisible|wsBorder|wsVScroll|wsHScroll|esMultiline|esReadonly|esAutovscroll|wsTabstop, 0, 0, 0, 0, parent, uintptr(idContent), inst, 0)
	a.btnPrimary, _ = createWindow(0, "BUTTON", "", wsChild|wsVisible|wsTabstop|bsPushbutton, 0, 0, 0, 0, parent, uintptr(idBtnPrimary), inst, 0)
	a.btnSecond, _ = createWindow(0, "BUTTON", "", wsChild|wsVisible|wsTabstop|bsPushbutton, 0, 0, 0, 0, parent, uintptr(idBtnSecondary), inst, 0)
	a.btnThird, _ = createWindow(0, "BUTTON", "", wsChild|wsVisible|wsTabstop|bsPushbutton, 0, 0, 0, 0, parent, uintptr(idBtnTertiary), inst, 0)
	a.applyFonts()
}

func (a *app) populateNav() {
	const lbAddstring = 0x0180
	const lbSetcursel = 0x0186
	for _, id := range a.screens {
		sendMessage(a.nav, lbAddstring, 0, uintptr(unsafe.Pointer(utf16ptr(a.bundle.T(present.NavKey(id))))))
	}
	sendMessage(a.nav, lbSetcursel, 0, 0)
}

func (a *app) layout() {
	var rc rect
	procGetClientRect.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&rc)))
	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)
	out := layout.Compute(layout.Input{
		ClientW:   w,
		ClientH:   h,
		DPI:       int(a.dpi),
		BtnCount:  a.visibleBtn,
		ShowBrand: true,
	})
	move := func(hwnd windows.HWND, r layout.Rect) {
		if hwnd == 0 {
			return
		}
		procMoveWindow.Call(uintptr(hwnd), uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 1)
	}
	move(a.brand, out.Brand)
	move(a.nav, out.Nav)
	move(a.content, out.Content)
	btns := []*windows.HWND{&a.btnPrimary, &a.btnSecond, &a.btnThird}
	for i, b := range btns {
		if i < len(out.Buttons) {
			move(*b, out.Buttons[i])
			procEnableWindow.Call(uintptr(*b), 1)
		} else {
			procMoveWindow.Call(uintptr(*b), 0, 0, 0, 0, 1)
		}
	}
}

func (a *app) onCommand(wParam uintptr) {
	id := loWord(wParam)
	notify := hiWord(wParam)
	switch id {
	case idNavList:
		if notify == lbnSelchange {
			const lbGetcursel = 0x0188
			sel := int(sendMessage(a.nav, lbGetcursel, 0, 0))
			if sel >= 0 && sel < len(a.screens) {
				a.current = a.screens[sel]
				a.refreshContent()
			}
		}
	case idBtnPrimary:
		if notify == bnClicked {
			a.onPrimary()
		}
	case idBtnSecondary:
		if notify == bnClicked {
			a.onSecondary()
		}
	case idBtnTertiary:
		if notify == bnClicked {
			a.onTertiary()
		}
	}
}

func (a *app) refreshContent() {
	a.mu.Lock()
	model := a.model
	screen := a.current
	a.mu.Unlock()
	text := present.Render(a.bundle, screen, model)
	setWindowText(a.content, text)
	a.updateButtons(screen)
}

func (a *app) updateButtons(screen string) {
	setWindowText(a.btnPrimary, "")
	setWindowText(a.btnSecond, "")
	setWindowText(a.btnThird, "")
	procEnableWindow.Call(uintptr(a.btnPrimary), 0)
	procEnableWindow.Call(uintptr(a.btnSecond), 0)
	procEnableWindow.Call(uintptr(a.btnThird), 0)

	labels := make([]string, 0, 3)
	enable := func(key string) {
		labels = append(labels, a.bundle.T(key))
	}

	switch screen {
	case present.ScreenDiagnostics, present.ScreenProgress:
		enable("action.start_diagnostics")
		enable("action.open_journal")
	case present.ScreenExport:
		enable("action.export_report")
		enable("action.select_folder")
	case present.ScreenJournal:
		enable("action.open_journal")
	case present.ScreenTools:
		enable("action.open_cmd")
	case present.ScreenPower:
		enable("action.reboot")
		enable("action.shutdown")
	case present.ScreenOverview, present.ScreenSummary:
		enable("action.start_diagnostics")
		enable("action.export_report")
		if a.kiosk {
			enable("action.windowed")
		} else {
			enable("action.fullscreen")
		}
	}
	btns := []windows.HWND{a.btnPrimary, a.btnSecond, a.btnThird}
	for i, label := range labels {
		setWindowText(btns[i], label)
		procEnableWindow.Call(uintptr(btns[i]), 1)
	}
	a.visibleBtn = len(labels)
	a.layout()
}

func (a *app) onPrimary() {
	switch a.current {
	case present.ScreenDiagnostics, present.ScreenProgress, present.ScreenOverview, present.ScreenSummary:
		a.startDiagnostics()
	case present.ScreenExport:
		a.doExport()
	case present.ScreenJournal:
		a.openJournalExternal()
	case present.ScreenTools:
		a.openCmd()
	case present.ScreenPower:
		a.confirmPower(true)
	}
}

func (a *app) onSecondary() {
	switch a.current {
	case present.ScreenDiagnostics, present.ScreenProgress:
		a.current = present.ScreenJournal
		a.selectNav(present.ScreenJournal)
		a.refreshContent()
	case present.ScreenExport:
		dir := a.pickExportDir()
		if dir != "" {
			a.mu.Lock()
			a.model.Export.TargetDir = dir
			a.mu.Unlock()
			a.refreshContent()
		}
	case present.ScreenOverview, present.ScreenSummary:
		a.current = present.ScreenExport
		a.selectNav(present.ScreenExport)
		a.refreshContent()
	case present.ScreenPower:
		a.confirmPower(false)
	}
}

func (a *app) onTertiary() {
	switch a.current {
	case present.ScreenOverview, present.ScreenSummary:
		a.applyKiosk(!a.kiosk)
		a.refreshContent()
	}
}

func (a *app) selectNav(screen string) {
	const lbSetcursel = 0x0186
	for i, id := range a.screens {
		if id == screen {
			sendMessage(a.nav, lbSetcursel, uintptr(i), 0)
			return
		}
	}
}

// queueProgress is safe to call from a background goroutine: it only updates
// the thread-safe queue and wakes the UI thread via PostMessage.
func (a *app) queueProgress(p viewmodel.ProgressScreen) {
	a.queue.PushProgress(p)
	postMessage(a.hwnd, msgUIProgress, 0, 0)
}

// queueDone is safe to call from a background goroutine: it only updates the
// thread-safe queue and wakes the UI thread via PostMessage. HWND mutations
// happen later on the UI thread inside drainUIEvents.
func (a *app) queueDone(res orchestrator.Result) {
	a.queue.PushTerminal(res)
	postMessage(a.hwnd, msgUIDone, 0, 0)
}

func (a *app) drainUIEvents() {
	progress, terminal := a.queue.Drain()
	if progress != nil {
		a.mu.Lock()
		a.model.Progress = *progress
		a.mu.Unlock()
		if a.current != present.ScreenProgress {
			a.current = present.ScreenProgress
			a.selectNav(present.ScreenProgress)
		}
		a.refreshContent()
	}
	if terminal == nil {
		return
	}
	a.mu.Lock()
	a.running = false
	res := *terminal
	if res.Code == orchestrator.ExitOK {
		a.model = res.Model
		a.model.Progress = viewmodel.ProgressScreen{Phase: "done", StatusKey: "status.succeeded", Percent: 100, Detail: "msg.collection_done"}
	} else {
		a.model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: res.FriendlyKey, FriendlyError: res.FriendlyKey, ShowJournalHint: true,
		}
		if res.Report != nil {
			a.model.Overview = res.Model.Overview
			a.model.Summary = res.Model.Summary
			a.model.Hardware = res.Model.Hardware
			a.model.Storage = res.Model.Storage
			a.model.BitLocker = res.Model.BitLocker
			a.model.Windows = res.Model.Windows
			a.model.Network = res.Model.Network
			a.model.Agent = res.Model.Agent
			a.model.Export = res.Model.Export
		}
		if a.cfg.Journal != nil {
			a.model.Journal.Entries = a.cfg.Journal.Entries()
		}
	}
	a.mu.Unlock()
	a.refreshContent()
	if res.Code != orchestrator.ExitOK {
		messageBox(a.hwnd, a.bundle.T(res.FriendlyKey)+"\n"+a.bundle.T("msg.see_journal"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
	} else {
		a.current = present.ScreenSummary
		a.selectNav(present.ScreenSummary)
		a.refreshContent()
	}
}

func (a *app) startDiagnostics() {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	if a.cfg.Mock || a.model.MockMode {
		a.mu.Unlock()
		a.appendJournal("mock diagnostics requested; mock data already loaded")
		a.mu.Lock()
		a.model.Progress = viewmodel.ProgressScreen{Phase: "done", StatusKey: "status.succeeded", Percent: 100, Detail: "msg.collection_done"}
		a.mu.Unlock()
		a.current = present.ScreenProgress
		a.selectNav(present.ScreenProgress)
		a.refreshContent()
		return
	}
	if a.cfg.Orchestrator == nil {
		a.mu.Unlock()
		messageBox(a.hwnd, a.bundle.T("msg.collector_missing"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return
	}
	a.running = true
	// Each diagnostic run needs a fresh queue; otherwise terminalSet from the
	// previous run permanently blocks progress/terminal delivery.
	a.queue.Reset()
	a.mu.Unlock()

	a.current = present.ScreenProgress
	a.selectNav(present.ScreenProgress)
	a.queueProgress(viewmodel.ProgressScreen{
		Phase: "collector", StatusKey: "status.running", Percent: 5, Detail: "msg.collection_running",
	})

	go func() {
		res := a.cfg.Orchestrator.RunCollection(context.Background(), func(p viewmodel.ProgressScreen) {
			a.queueProgress(p)
		})
		a.queueDone(res)
	}()
}

func (a *app) exportPolicy() export.ExportPolicy {
	a.mu.Lock()
	defer a.mu.Unlock()
	var installRoots []string
	for _, inst := range a.model.Windows.Installs {
		installRoots = append(installRoots, inst.Root)
	}
	return export.ExportPolicy{ExcludeRoots: export.DefaultExcludeRoots(installRoots)}
}

// authorizeExportPath applies ExportPolicy to a concrete path from the folder
// dialog or a previously stored target. Nested folders cannot bypass volume rules.
func (a *app) authorizeExportPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	eval, err := export.EvaluateExportPath(path, export.OSDriveScanner{}, a.exportPolicy())
	if err != nil {
		messageBox(a.hwnd, a.bundle.T("msg.export_usb_failed"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return ""
	}
	switch eval.Decision {
	case export.PathAllowRemovable:
		return path
	case export.PathRequireFixedConfirm:
		msg := a.bundle.T("msg.export_fixed_confirm") + "\n" + export.FormatDrive(eval.Drive) + "\n" + path
		if messageBox(a.hwnd, msg, a.bundle.T("app.brand"), mbYesNo|mbIconWarn) != idYes {
			return ""
		}
		if messageBox(a.hwnd, a.bundle.T("msg.export_fixed_confirm"), a.bundle.T("app.brand"), mbOKCancel|mbIconWarn) != idOK {
			return ""
		}
		return path
	case export.PathRejectExcluded:
		key := eval.ReasonKey
		if key == "" {
			key = "msg.export_path_excluded"
		}
		messageBox(a.hwnd, a.bundle.T(key)+"\n"+path, a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return ""
	default:
		key := eval.ReasonKey
		if key == "" {
			key = "msg.export_path_unknown"
		}
		messageBox(a.hwnd, a.bundle.T(key)+"\n"+path, a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return ""
	}
}

func (a *app) pickExportDir() string {
	dir := browseForFolder(a.hwnd, a.bundle.T("action.select_folder"))
	if dir != "" {
		return a.authorizeExportPath(dir)
	}
	policy := a.exportPolicy()
	removable, fixed, err := export.CandidateDrives(export.OSDriveScanner{}, policy)
	if err != nil {
		messageBox(a.hwnd, a.bundle.T("msg.export_usb_failed"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return ""
	}
	choose := func(list []export.DriveInfo, requireConfirm bool) string {
		for _, d := range list {
			msg := a.bundle.T("msg.export_pick_drive") + "\n" + export.FormatDrive(d) + "\n" + export.ExportDirForDrive(d.Root)
			if requireConfirm {
				msg = a.bundle.T("msg.export_fixed_confirm") + "\n" + export.FormatDrive(d)
			}
			if messageBox(a.hwnd, msg, a.bundle.T("app.brand"), mbYesNo|mbIconWarn) == idYes {
				if requireConfirm {
					// Second explicit confirmation for fixed/internal disks.
					if messageBox(a.hwnd, a.bundle.T("msg.export_fixed_confirm"), a.bundle.T("app.brand"), mbOKCancel|mbIconWarn) != idOK {
						continue
					}
				}
				return export.ExportDirForDrive(d.Root)
			}
		}
		return ""
	}
	if dir := choose(removable, false); dir != "" {
		return dir
	}
	if len(fixed) == 0 {
		messageBox(a.hwnd, a.bundle.T("msg.export_usb_failed"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
		return ""
	}
	if messageBox(a.hwnd, a.bundle.T("msg.export_no_removable"), a.bundle.T("app.brand"), mbYesNo|mbIconWarn) != idYes {
		return ""
	}
	return choose(fixed, true)
}

func (a *app) doExport() {
	a.mu.Lock()
	target := a.model.Export.TargetDir
	sources := map[string]string{
		filepath.Base(a.model.Export.ReportPath):    a.model.Export.ReportPath,
		filepath.Base(a.model.Export.DiagnosisPath): a.model.Export.DiagnosisPath,
		filepath.Base(a.model.Export.SessionPath):   a.model.Export.SessionPath,
		filepath.Base(a.model.Export.JournalPath):   a.model.Export.JournalPath,
	}
	a.mu.Unlock()
	if target == "" {
		target = a.pickExportDir()
		if target == "" {
			return
		}
	} else {
		target = a.authorizeExportPath(target)
		if target == "" {
			return
		}
	}
	if a.cfg.Mock || a.model.MockMode {
		sources = materializeMockExport()
	}
	cleaned := make(map[string]string, len(sources))
	for name, src := range sources {
		if src == "" || (len(src) > 0 && src[0] == '(') {
			continue
		}
		cleaned[name] = src
	}
	res := export.CopySession(target, cleaned)
	a.mu.Lock()
	a.model.Export.TargetDir = target
	a.model.Export.LastOK = res.OK
	a.model.Export.LastMessage = res.FriendlyKey
	a.mu.Unlock()
	a.appendJournal("export: %s (%s)", res.FriendlyKey, res.Detail)
	a.refreshContent()
	flags := uint32(mbOK)
	if !res.OK {
		flags |= mbIconWarn
	}
	messageBox(a.hwnd, a.bundle.T(res.FriendlyKey), a.bundle.T("app.brand"), flags)
}

func (a *app) openCmd() {
	candidates := []string{
		filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"),
		`X:\Windows\System32\cmd.exe`,
		"cmd.exe",
	}
	var lastErr error
	for _, c := range candidates {
		cmd := exec.Command(c)
		if err := cmd.Start(); err == nil {
			a.appendJournal("opened %s", c)
			return
		} else {
			lastErr = err
		}
	}
	messageBox(a.hwnd, a.bundle.T("msg.cmd_failed"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
	a.appendJournal("open cmd failed: %v", lastErr)
}

func (a *app) openJournalExternal() {
	a.mu.Lock()
	path := a.model.Export.JournalPath
	a.mu.Unlock()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := exec.Command("notepad.exe", path).Start(); err == nil {
				return
			}
			a.appendJournal("notepad.exe unavailable; using built-in journal viewer")
		}
	}
	a.current = present.ScreenJournal
	a.selectNav(present.ScreenJournal)
	a.refreshContent()
}

func (a *app) confirmPower(reboot bool) {
	key := "msg.confirm_shutdown"
	if reboot {
		key = "msg.confirm_reboot"
	}
	if messageBox(a.hwnd, a.bundle.T(key), a.bundle.T("app.brand"), mbOKCancel|mbIconWarn) != idOK {
		return
	}
	flags := uint32(ewxShutdown | ewxForce)
	wpeutilArg := "shutdown"
	if reboot {
		flags = ewxReboot | ewxForce
		wpeutilArg = "reboot"
	}
	r, _, err := procExitWindowsEx.Call(uintptr(flags), 0)
	if r != 0 {
		return
	}
	a.appendJournal("ExitWindowsEx failed: %v; trying wpeutil %s", err, wpeutilArg)
	cmd := exec.Command("wpeutil", wpeutilArg)
	if err := cmd.Start(); err != nil {
		a.appendJournal("wpeutil %s failed: %v", wpeutilArg, err)
		messageBox(a.hwnd, a.bundle.T("msg.process_failed"), a.bundle.T("app.brand"), mbOK|mbIconWarn)
	}
}

func (a *app) applyKiosk(enable bool) {
	a.kiosk = enable
	style := getWindowLongPtr(a.hwnd, gwlStyle)
	if enable {
		var rc rect
		procGetClientRect.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&rc)))
		// Preserve current restored size roughly.
		screenW, _, _ := procGetSystemMetrics.Call(smCXscreen)
		screenH, _, _ := procGetSystemMetrics.Call(smCYscreen)
		style &^= uintptr(wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu)
		setWindowLongPtr(a.hwnd, gwlStyle, style)
		setWindowPos(a.hwnd, 0, 0, int32(screenW), int32(screenH), swpFrameChanged|swpShowWindow)
		a.appendJournal("entered fullscreen/kiosk mode")
	} else {
		style |= uintptr(wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu)
		setWindowLongPtr(a.hwnd, gwlStyle, style|uintptr(wsOverlappedWindow))
		setWindowPos(a.hwnd, a.windowedX, a.windowedY, a.windowedW, a.windowedH, swpFrameChanged|swpShowWindow)
		procShowWindow.Call(uintptr(a.hwnd), swRestore)
		a.appendJournal("entered windowed mode")
	}
	a.layout()
}

func (a *app) appendJournal(format string, args ...any) {
	if a.cfg.Journal != nil {
		a.cfg.Journal.Append(format, args...)
		a.mu.Lock()
		a.model.Journal.Entries = a.cfg.Journal.Entries()
		a.mu.Unlock()
	}
}

func materializeMockExport() map[string]string {
	tmp := filepath.Join(os.TempDir(), "effexorwinpe-mock-export")
	_ = os.MkdirAll(tmp, 0o755)
	files := map[string][]byte{
		"initial.json":                   mock.ReportJSON(),
		"initial-diagnosis.json":         mock.DiagnosisJSON(),
		"initial-diagnosis-session.json": mock.SessionJSON(),
	}
	out := make(map[string]string, len(files))
	for name, raw := range files {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, raw, 0o644); err == nil {
			out[name] = path
		}
	}
	return out
}
