//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	WS_OVERLAPPEDWINDOW          = 0x00CF0000
	WS_VISIBLE                   = 0x10000000
	WS_CHILD                     = 0x40000000
	WS_TABSTOP                   = 0x00010000
	WS_BORDER                    = 0x00800000
	WS_VSCROLL                   = 0x00200000
	ES_AUTOHSCROLL               = 0x0080
	EM_SETMARGINS                = 0x00D3
	EC_LEFTMARGIN                = 0x0001
	EC_RIGHTMARGIN               = 0x0002
	ES_LEFT                      = 0
	CBS_DROPDOWNLIST             = 0x0003
	CBS_HASSTRINGS               = 0x0200
	BS_PUSHBUTTON                = 0
	BS_OWNERDRAW                 = 0x0000000B
	LVS_REPORT                   = 0x0001
	LVS_SHOWSELALWAYS            = 0x0008
	LVS_OWNERDATA                = 0x1000
	LVS_EX_FULLROWSELECT         = 0x20
	LVS_EX_GRIDLINES             = 0x1
	LVS_EX_DOUBLEBUFFER          = 0x00010000
	LVSICF_NOINVALIDATEALL       = 0x1
	LVSICF_NOSCROLL              = 0x2
	LVCF_FMT                     = 0x1
	LVCF_WIDTH                   = 0x2
	LVCF_TEXT                    = 0x4
	LVCF_SUBITEM                 = 0x8
	LVCFMT_LEFT                  = 0
	LVCFMT_RIGHT                 = 1
	LVM_FIRST                    = 0x1000
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_SETBKCOLOR               = LVM_FIRST + 1
	LVM_SETTEXTCOLOR             = LVM_FIRST + 36
	LVM_SETTEXTBKCOLOR           = LVM_FIRST + 38
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_SETITEMCOUNT             = LVM_FIRST + 47
	LVM_SETCOLUMNWIDTH           = LVM_FIRST + 30
	LVM_GETNEXTITEM              = LVM_FIRST + 12
	LVM_HITTEST                  = LVM_FIRST + 18
	LVM_REDRAWITEMS              = LVM_FIRST + 21
	LVM_GETTOPINDEX              = LVM_FIRST + 39
	LVM_GETCOUNTPERPAGE          = LVM_FIRST + 40
	LVNI_SELECTED                = 0x2
	LVHT_ONITEM                  = 0x000E
	LVIF_TEXT                    = 0x0001
	LVN_FIRST                    = -100
	LVN_COLUMNCLICK              = LVN_FIRST - 8
	LVN_GETDISPINFOW             = LVN_FIRST - 77
	NM_DBLCLK                    = -3
	NM_RCLICK                    = -5
	WM_CREATE                    = 0x0001
	WM_ERASEBKGND                = 0x0014
	WM_PAINT                     = 0x000F
	WM_ENTERSIZEMOVE             = 0x0231
	WM_EXITSIZEMOVE              = 0x0232
	WM_DRAWITEM                  = 0x002B
	WM_CTLCOLORBTN               = 0x0135
	WM_CTLCOLOREDIT              = 0x0133
	WM_CTLCOLORSTATIC            = 0x0138
	WM_DESTROY                   = 0x0002
	WM_SIZE                      = 0x0005
	WM_TIMER                     = 0x0113
	WM_SETFOCUS                  = 0x0007
	WM_COMMAND                   = 0x0111
	WM_NOTIFY                    = 0x004E
	WM_CONTEXTMENU               = 0x007B
	WM_DPICHANGED                = 0x02E0
	WM_APP                       = 0x8000
	WM_APP_REFRESH               = WM_APP + 1
	WM_APP_STATUS                = WM_APP + 2
	WM_APP_SORT_DONE             = WM_APP + 3
	WM_APP_SEARCH_DONE           = WM_APP + 4
	WM_APP_ROW_REDRAW            = WM_APP + 5
	WM_APP_DRIVES_DONE           = WM_APP + 6
	WM_SETFONT                   = 0x0030
	WM_GETTEXT                   = 0x000D
	CB_ADDSTRING                 = 0x0143
	CB_RESETCONTENT              = 0x014B
	CB_GETCURSEL                 = 0x0147
	CB_SETCURSEL                 = 0x014E
	CB_GETLBTEXT                 = 0x0148
	CB_SETDROPPEDWIDTH           = 0x0160
	BN_CLICKED                   = 0
	CBN_SELCHANGE                = 1
	IDC_ARROW                    = 32512
	SW_SHOWNORMAL                = 1
	SW_SHOW                      = 5
	SWP_NOZORDER                 = 0x0004
	SWP_NOACTIVATE               = 0x0010
	SIZE_MINIMIZED               = 1
	MF_STRING                    = 0x0000
	TPM_RETURNCMD                = 0x0100
	TPM_RIGHTBUTTON              = 0x0002
	CF_UNICODETEXT               = 13
	GMEM_MOVEABLE                = 0x0002
	OPEN_EXISTING                = 3
	GENERIC_READ                 = 0x80000000
	FILE_SHARE_READ              = 1
	FILE_SHARE_WRITE             = 2
	FILE_SHARE_DELETE            = 4
	INVALID_HANDLE_VALUE         = ^uintptr(0)
	FSCTL_ENUM_USN_DATA          = 0x000900B3
	FSCTL_QUERY_USN_JOURNAL      = 0x000900F4
	FSCTL_READ_USN_JOURNAL       = 0x000900BB
	FILE_ATTRIBUTE_DIRECTORY     = 0x10
	MAX_RESULTS                  = 500000
)

var (
	user32                            = syscall.NewLazyDLL("user32.dll")
	kernel32                          = syscall.NewLazyDLL("kernel32.dll")
	gdi32                             = syscall.NewLazyDLL("gdi32.dll")
	comctl32                          = syscall.NewLazyDLL("comctl32.dll")
	shell32                           = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procUpdateWindow                  = user32.NewProc("UpdateWindow")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procSetTimer                      = user32.NewProc("SetTimer")
	procKillTimer                     = user32.NewProc("KillTimer")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procLoadIconW                     = user32.NewProc("LoadIconW")
	procGetModuleHandleW              = kernel32.NewProc("GetModuleHandleW")
	procMoveWindow                    = user32.NewProc("MoveWindow")
	procBeginDeferWindowPos           = user32.NewProc("BeginDeferWindowPos")
	procDeferWindowPos                = user32.NewProc("DeferWindowPos")
	procEndDeferWindowPos             = user32.NewProc("EndDeferWindowPos")
	procBeginPaint                    = user32.NewProc("BeginPaint")
	procEndPaint                      = user32.NewProc("EndPaint")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procSendMessageW                  = user32.NewProc("SendMessageW")
	procSetWindowTextW                = user32.NewProc("SetWindowTextW")
	procGetWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW                = user32.NewProc("GetWindowTextW")
	procEnableWindow                  = user32.NewProc("EnableWindow")
	procSetFocus                      = user32.NewProc("SetFocus")
	procInvalidateRect                = user32.NewProc("InvalidateRect")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procCreateFontW                   = gdi32.NewProc("CreateFontW")
	procDeleteObject                  = gdi32.NewProc("DeleteObject")
	procCreateSolidBrush              = gdi32.NewProc("CreateSolidBrush")
	procCreatePen                     = gdi32.NewProc("CreatePen")
	procSelectObject                  = gdi32.NewProc("SelectObject")
	procSetTextColor                  = gdi32.NewProc("SetTextColor")
	procSetBkColor                    = gdi32.NewProc("SetBkColor")
	procSetBkMode                     = gdi32.NewProc("SetBkMode")
	procRoundRect                     = gdi32.NewProc("RoundRect")
	procFillRect                      = user32.NewProc("FillRect")
	procDrawTextW                     = user32.NewProc("DrawTextW")
	procInitCommonControlsEx          = comctl32.NewProc("InitCommonControlsEx")
	procCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	procAppendMenuW                   = user32.NewProc("AppendMenuW")
	procTrackPopupMenu                = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                   = user32.NewProc("DestroyMenu")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procShellExecuteW                 = shell32.NewProc("ShellExecuteW")
	procOpenClipboard                 = user32.NewProc("OpenClipboard")
	procEmptyClipboard                = user32.NewProc("EmptyClipboard")
	procSetClipboardData              = user32.NewProc("SetClipboardData")
	procCloseClipboard                = user32.NewProc("CloseClipboard")
	procGlobalAlloc                   = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock                  = kernel32.NewProc("GlobalUnlock")
	procGetLogicalDrives              = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW                 = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformationW         = kernel32.NewProc("GetVolumeInformationW")
	procCreateFileW                   = kernel32.NewProc("CreateFileW")
	procDeviceIoControl               = kernel32.NewProc("DeviceIoControl")
	procCloseHandle                   = kernel32.NewProc("CloseHandle")
	procGetFileAttributesExW          = kernel32.NewProc("GetFileAttributesExW")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetModuleFileNameW            = kernel32.NewProc("GetModuleFileNameW")
	procIsUserAnAdmin                 = shell32.NewProc("IsUserAnAdmin")
	procSetWindowTheme                = syscall.NewLazyDLL("uxtheme.dll").NewProc("SetWindowTheme")
)

type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	Private uint32
}
type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	HDC       uintptr
	Erase     int32
	Paint     RECT
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}
type NMHDR struct {
	HwndFrom uintptr
	IDFrom   uintptr
	Code     int32
}
type LVITEMW struct {
	Mask       uint32
	IItem      int32
	ISubItem   int32
	State      uint32
	StateMask  uint32
	Text       uintptr
	CchTextMax int32
	IImage     int32
	LParam     uintptr
	IIndent    int32
	IGroupID   int32
	CColumns   uint32
	PColumns   uintptr
	PIColFmt   uintptr
	IGroup     int32
}
type NMLVDISPINFO struct {
	Hdr  NMHDR
	Item LVITEMW
}
type NMLISTVIEW struct {
	Hdr       NMHDR
	IItem     int32
	ISubItem  int32
	UNewState uint32
	UOldState uint32
	UChanged  uint32
	PtAction  POINT
	LParam    uintptr
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     RECT
	ItemData   uintptr
}
type LVHITTESTINFO struct {
	Pt       POINT
	Flags    uint32
	IItem    int32
	ISubItem int32
	IGroup   int32
}
type LVCOLUMNW struct {
	Mask       uint32
	Fmt        int32
	CX         int32
	Text       uintptr
	CchTextMax int32
	ISubItem   int32
	IImage     int32
	IOrder     int32
	CXMin      int32
	CXDefault  int32
	CXIdeal    int32
}
type INITCOMMONCONTROLSEX struct {
	Size uint32
	ICC  uint32
}
type FILETIME struct {
	Low  uint32
	High uint32
}
type WIN32_FILE_ATTRIBUTE_DATA struct {
	Attr     uint32
	Creation FILETIME
	Access   FILETIME
	Write    FILETIME
	SizeHigh uint32
	SizeLow  uint32
}
type USN_ENUM_DATA struct {
	StartFRN uint64
	LowUSN   int64
	HighUSN  int64
}
type READ_USN_JOURNAL_DATA struct {
	StartUSN        int64
	ReasonMask      uint32
	ReturnOnlyClose uint32
	Timeout         uint64
	BytesToWaitFor  uint64
	JournalID       uint64
}
type USN_JOURNAL_DATA struct {
	JournalID       uint64
	FirstUSN        int64
	NextUSN         int64
	LowestValidUSN  int64
	MaxUSN          int64
	MaximumSize     int64
	AllocationDelta int64
	MinMajor        uint16
	MaxMajor        uint16
	Flags           uint32
	RangeChunk      uint64
	RangeThreshold  int64
}

type Entry struct {
	ID        uint64
	Parent    uint64
	Attr      uint32
	Name      string
	LowerName string
}
type DirNode struct {
	Parent uint64
	Name   string
}
type FileResult struct {
	Name, Type, Path            string
	NameKey                     string
	Size                        int64
	Created, Modified, Accessed time.Time
	IsDir                       bool
	MetaLoaded, MetaLoading     bool
}
type VolumeIndex struct {
	Serial    uint32
	JournalID uint64
	NextUSN   int64
	Entries   []Entry
	Dirs      map[uint64]DirNode
}
type App struct {
	mu                                                                               sync.RWMutex
	sortMu                                                                           sync.Mutex
	main                                                                             uintptr
	list                                                                             uintptr
	query, drive, kind, include, exclude, searchBtn, cancelBtn, lang, status, header uintptr
	queryLabel, rangeLabel, kindLabel, includeLabel, excludeLabel                    uintptr
	controls                                                                         []uintptr
	font, inputFont, queryFont, headerFont                                           uintptr
	bgBrush, cardBrush, headerBrush, cardPen                                         uintptr
	dpi                                                                              int32
	inSizeMove                                                                       bool
	pathColWidth                                                                     int32
	english                                                                          bool
	drives                                                                           []string
	drivesLoading                                                                    bool
	indices                                                                          map[string]*VolumeIndex
	results                                                                          []FileResult
	searching                                                                        bool
	sorting                                                                          bool
	searchCancel                                                                     context.CancelFunc
	generation                                                                       int64
	sortCol                                                                          int
	sortAsc                                                                          bool
	pendingSorted                                                                    []FileResult
	statusText                                                                       string
	resultsDone                                                                      bool
	resultsDirty                                                                     bool
	lastShownCount                                                                   int
	dispText                                                                         []uint16
}

var app = &App{indices: make(map[string]*VolumeIndex), sortCol: -1, sortAsc: true}

func wptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func callText(hwnd uintptr, s string) {
	p := wptr(s)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(p)))
}
func getText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	b := make([]uint16, int(n)+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func send(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(hwnd, uintptr(msg), wp, lp)
	return r
}
func createControl(class, text string, style uint32, x, y, w, h int32, id uintptr, ex uint32) uintptr {
	cp, tp := wptr(class), wptr(text)
	inst, _, _ := procGetModuleHandleW.Call(0)
	r, _, _ := procCreateWindowExW.Call(uintptr(ex), uintptr(unsafe.Pointer(cp)), uintptr(unsafe.Pointer(tp)), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), app.main, id, inst, 0)
	if r != 0 {
		app.controls = append(app.controls, r)
		if app.font != 0 {
			send(r, WM_SETFONT, app.font, 1)
		}
	}
	return r
}
func wndProc(hwnd uintptr, msg uintptr, wp uintptr, lp uintptr) uintptr {
	switch uint32(msg) {
	case WM_ERASEBKGND:
		// Background is painted in WM_PAINT. Claim erasure here to avoid a white
		// erase followed by a second paint pass while the user resizes the window.
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			app.paintBackground(hdc)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case WM_CTLCOLORSTATIC:
		hdc, child := wp, lp
		procSetBkMode.Call(hdc, 1) // transparent static text over the painted panel
		if child == app.header {
			procSetTextColor.Call(hdc, rgb(255, 255, 255))
			return app.headerBrush
		}
		if child == app.status {
			procSetTextColor.Call(hdc, rgb(89, 101, 119))
			return app.bgBrush
		}
		procSetTextColor.Call(hdc, rgb(55, 65, 81))
		return app.cardBrush
	case WM_CTLCOLOREDIT:
		hdc := wp
		procSetBkMode.Call(hdc, 2) // opaque edit background
		procSetTextColor.Call(hdc, rgb(31, 41, 55))
		procSetBkColor.Call(hdc, rgb(255, 255, 255))
		return app.cardBrush
	case WM_DRAWITEM:
		if lp != 0 {
			app.drawButton((*DRAWITEMSTRUCT)(unsafe.Pointer(lp)))
			return 1
		}
	case WM_CREATE:
		app.main = hwnd
		app.dpi = 96
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
			app.dpi = int32(d)
		}
		app.resetCardPen()
		app.makeFonts()
		app.createUI()
		app.layout()
		// Coalesce background result updates into one UI refresh every 100 ms.
		procSetTimer.Call(hwnd, 1, 100, 0)
	case WM_SIZE:
		if wp != SIZE_MINIMIZED {
			app.layout()
			// Keep the canvas/header fully painted as the client area changes.
			procInvalidateRect.Call(hwnd, 0, 0)
		}
	case WM_ENTERSIZEMOVE:
		app.inSizeMove = true
	case WM_EXITSIZEMOVE:
		app.inSizeMove = false
		// Defer the expensive last-column width update until the drag ends.
		app.layout()
		procInvalidateRect.Call(hwnd, 0, 0)
	case WM_COMMAND:
		id := uint16(wp & 0xffff)
		code := uint16((wp >> 16) & 0xffff)
		switch id {
		case 1003:
			if code == BN_CLICKED {
				app.startSearch()
			}
		case 1004:
			if code == BN_CLICKED {
				app.cancelSearchNow()
			}
		case 1006:
			if code == CBN_SELCHANGE {
				app.changeLanguage()
			}
		case 1001:
			if code == 0 {
			}
		}
	case WM_NOTIFY:
		if lp != 0 {
			hdr := (*NMHDR)(unsafe.Pointer(lp))
			switch hdr.Code {
			case LVN_GETDISPINFOW:
				info := (*NMLVDISPINFO)(unsafe.Pointer(lp))
				if info.Item.Mask&LVIF_TEXT != 0 {
					txt := app.cellText(int(info.Item.IItem), int(info.Item.ISubItem))
					p, _ := syscall.UTF16FromString(txt)
					app.dispText = p // retain callback text until the list view consumes it
					if len(app.dispText) > 0 {
						info.Item.Text = uintptr(unsafe.Pointer(&app.dispText[0]))
					} else {
						info.Item.Text = 0
					}
				}
			case LVN_COLUMNCLICK:
				n := (*NMLISTVIEW)(unsafe.Pointer(lp))
				app.sortByColumn(int(n.ISubItem))
			case NM_RCLICK:
				if hdr.HwndFrom == app.list {
					app.showContextMenu()
				}
			case NM_DBLCLK:
				n := (*NMLISTVIEW)(unsafe.Pointer(lp))
				if n.IItem >= 0 {
					app.openResult(int(n.IItem), false)
				}
			}
		}
	case WM_CONTEXTMENU:
		if uintptr(wp) == app.list {
			app.showContextMenu()
		}
	case WM_APP_REFRESH:
		app.refreshListCount()
	case WM_TIMER:
		if wp == 1 {
			app.mu.Lock()
			dirty := app.resultsDirty
			app.resultsDirty = false
			app.mu.Unlock()
			if dirty {
				app.refreshListCount()
			}
		}
	case WM_APP_ROW_REDRAW:
		row := int(wp)
		if row >= 0 {
			send(app.list, LVM_REDRAWITEMS, uintptr(row), uintptr(row))
		}
	case WM_APP_DRIVES_DONE:
		app.mu.Lock()
		drives := append([]string(nil), app.drives...)
		app.drivesLoading = false
		app.mu.Unlock()
		send(app.drive, CB_RESETCONTENT, 0, 0)
		vols := []string{app.localize("全部 NTFS 卷", "All NTFS volumes")}
		vols = append(vols, drives...)
		if len(drives) == 0 {
			vols = []string{app.localize("未找到 NTFS 卷", "No NTFS volumes found")}
		}
		app.addCombo(app.drive, vols)
		send(app.drive, CB_SETCURSEL, 0, 0)
		procEnableWindow.Call(app.searchBtn, uintptr(boolInt(len(drives) > 0)))
		if len(drives) == 0 {
			callText(app.status, app.localize("未找到可搜索的本地 NTFS 卷。", "No local NTFS volumes found."))
		} else {
			callText(app.status, app.localize("就绪", "Ready"))
		}
	case WM_APP_STATUS:
		app.mu.RLock()
		s := app.statusText
		app.mu.RUnlock()
		callText(app.status, s)
	case WM_APP_SORT_DONE:
		app.mu.Lock()
		if app.pendingSorted != nil {
			app.results = app.pendingSorted
			app.pendingSorted = nil
		}
		app.sorting = false
		app.mu.Unlock()
		app.refreshListCount()
		procInvalidateRect.Call(app.list, 0, 0) // double-buffered full repaint after order changes
		callText(app.status, app.localize("排序完成", "Sorted"))
	case WM_APP_SEARCH_DONE:
		app.mu.RLock()
		currentGen := app.generation
		app.mu.RUnlock()
		if int64(wp) != currentGen {
			return 0
		}
		app.mu.Lock()
		app.searching = false
		app.resultsDone = true
		app.resultsDirty = false
		app.mu.Unlock()
		procEnableWindow.Call(app.searchBtn, 1)
		procEnableWindow.Call(app.cancelBtn, 0)
		app.refreshListCount()
		app.mu.RLock()
		s := app.statusText
		app.mu.RUnlock()
		callText(app.status, s)
	case WM_DPICHANGED:
		if wp != 0 {
			app.dpi = int32(wp & 0xffff)
			app.resetFonts()
			app.resetCardPen()
			app.pathColWidth = 0
			app.layout()
			procInvalidateRect.Call(hwnd, 0, 0)
		}
	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		app.cancelSearchNow()
		for _, obj := range []uintptr{app.font, app.inputFont, app.queryFont, app.headerFont, app.bgBrush, app.cardBrush, app.headerBrush, app.cardPen} {
			if obj != 0 {
				procDeleteObject.Call(obj)
			}
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}
func (a *App) localize(cn, en string) string {
	if a.english {
		return en
	}
	return cn
}
func (a *App) makeFonts() { a.resetFonts() }
func (a *App) resetFonts() {
	if a.font != 0 {
		procDeleteObject.Call(a.font)
	}
	if a.inputFont != 0 {
		procDeleteObject.Call(a.inputFont)
	}
	if a.queryFont != 0 {
		procDeleteObject.Call(a.queryFont)
	}
	if a.headerFont != 0 {
		procDeleteObject.Call(a.headerFont)
	}
	h := func(pt int32) uintptr { return uintptr(-pt * a.dpi / 72) }
	face := wptr("Microsoft YaHei UI")
	// Larger point sizes are intentional: the earlier 14–18 pt controls looked tiny
	// beside their oversized edit rectangles on high-DPI displays.
	a.font, _, _ = procCreateFontW.Call(h(12), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	a.inputFont, _, _ = procCreateFontW.Call(h(12), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	a.queryFont, _, _ = procCreateFontW.Call(h(17), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	a.headerFont, _, _ = procCreateFontW.Call(h(18), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	for _, c := range a.controls {
		f := a.font
		if c == a.query {
			f = a.queryFont
		} else if c == a.include || c == a.exclude || c == a.drive || c == a.kind {
			f = a.inputFont
		}
		if c == a.header {
			f = a.headerFont
		}
		if f != 0 {
			send(c, WM_SETFONT, f, 1)
		}
	}
}
func (a *App) createUI() {
	// App bar
	a.header = createControl("STATIC", "SwiftSearch Plus", WS_CHILD|WS_VISIBLE, 0, 0, 320, 34, 1010, 0)
	// Query uses the standard single-line EDIT style and auto-horizontal scrolling.
	a.queryLabel = createControl("STATIC", a.localize("文件名或路径", "File name or path"), WS_CHILD|WS_VISIBLE, 0, 0, 1, 1, 1011, 0)
	a.query = createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL|ES_LEFT, 0, 0, 300, 40, 1001, 0)
	a.searchBtn = createControl("BUTTON", a.localize("搜索", "Search"), WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 110, 40, 1003, 0)
	a.cancelBtn = createControl("BUTTON", a.localize("取消", "Cancel"), WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 100, 40, 1004, 0)
	procEnableWindow.Call(a.cancelBtn, 0)
	a.rangeLabel = createControl("STATIC", a.localize("搜索范围", "Search in"), WS_CHILD|WS_VISIBLE, 0, 0, 1, 1, 1012, 0)
	a.drive = createControl("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS|WS_VSCROLL, 0, 0, 120, 240, 1002, 0)
	a.kindLabel = createControl("STATIC", a.localize("显示类型", "Show"), WS_CHILD|WS_VISIBLE, 0, 0, 1, 1, 1013, 0)
	a.kind = createControl("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS|WS_VSCROLL, 0, 0, 150, 240, 1005, 0)
	a.includeLabel = createControl("STATIC", a.localize("自定义扩展名", "Include"), WS_CHILD|WS_VISIBLE, 0, 0, 1, 1, 1014, 0)
	a.include = createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 0, 0, 180, 40, 1007, 0)
	a.excludeLabel = createControl("STATIC", a.localize("排除扩展名", "Exclude"), WS_CHILD|WS_VISIBLE, 0, 0, 1, 1, 1015, 0)
	a.exclude = createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 0, 0, 180, 40, 1008, 0)
	a.lang = createControl("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS|WS_VSCROLL, 0, 0, 110, 240, 1006, 0)
	a.status = createControl("STATIC", a.localize("就绪", "Ready"), WS_CHILD|WS_VISIBLE, 0, 0, 500, 26, 1016, 0)
	a.list = createControl("SysListView32", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|LVS_REPORT|LVS_OWNERDATA|LVS_SHOWSELALWAYS|WS_BORDER, 0, 0, 1000, 500, 1020, 0x00000200)
	send(a.list, LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER)
	send(a.list, LVM_SETBKCOLOR, 0, rgb(255, 255, 255))
	send(a.list, LVM_SETTEXTBKCOLOR, 0, rgb(255, 255, 255))
	send(a.list, LVM_SETTEXTCOLOR, 0, rgb(38, 50, 67))
	a.addCombo(a.lang, []string{"简体中文", "English"})
	send(a.lang, CB_SETCURSEL, 0, 0)
	a.addCombo(a.kind, []string{"全部文件", "仅文件", "仅文件夹", "图片", "视频", "音频", "文档", "压缩包", "程序/安装包"})
	send(a.kind, CB_SETCURSEL, 0, 0)
	a.mu.Lock()
	a.drivesLoading = true
	a.mu.Unlock()
	a.addCombo(a.drive, []string{a.localize("正在识别 NTFS 卷…", "Detecting NTFS volumes…")})
	send(a.drive, CB_SETCURSEL, 0, 0)
	procEnableWindow.Call(a.searchBtn, 0)
	go func() {
		drives := a.enumerateNTFSVolumes()
		a.mu.Lock()
		a.drives = drives
		a.mu.Unlock()
		procPostMessageW.Call(a.main, WM_APP_DRIVES_DONE, 0, 0)
	}()
	cols := []struct {
		name  string
		width int32
		fmt   int32
	}{{a.localize("名称", "Name"), 260, LVCFMT_LEFT}, {a.localize("类型", "Type"), 110, LVCFMT_LEFT}, {a.localize("大小", "Size"), 110, LVCFMT_RIGHT}, {a.localize("创建时间", "Created"), 160, LVCFMT_LEFT}, {a.localize("最后写入", "Modified"), 160, LVCFMT_LEFT}, {a.localize("上次访问", "Accessed"), 160, LVCFMT_LEFT}, {a.localize("完整路径", "Full path"), 480, LVCFMT_LEFT}}
	for i, c := range cols {
		p := wptr(c.name)
		col := LVCOLUMNW{Mask: LVCF_FMT | LVCF_WIDTH | LVCF_TEXT | LVCF_SUBITEM, Fmt: c.fmt, CX: c.width * a.dpi / 96, Text: uintptr(unsafe.Pointer(p)), CchTextMax: int32(len(utf16.Encode([]rune(c.name)))), ISubItem: int32(i)}
		send(a.list, LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
	a.resetFonts()
	// Explicit inner padding makes the larger single-line text visually centered and readable.
	margin := uintptr(a.dpi * 10 / 96)
	for _, edit := range []uintptr{a.query, a.include, a.exclude} {
		send(edit, EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, margin|(margin<<16))
	}
	// Explorer theme + double-buffering reduces classic control artifacts.
	for _, h := range []uintptr{a.list, a.drive, a.kind, a.lang} {
		ex := wptr("Explorer")
		procSetWindowTheme.Call(h, uintptr(unsafe.Pointer(ex)), 0)
	}
	a.layout()
}
func (a *App) addCombo(hwnd uintptr, items []string) {
	for _, s := range items {
		p := wptr(s)
		send(hwnd, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p)))
	}
	send(hwnd, CB_SETDROPPEDWIDTH, 400, 0)
}

type childLayout struct {
	hwnd       uintptr
	x, y, w, h int32
}

func (a *App) layout() {
	if a.main == 0 {
		return
	}
	var r RECT
	procGetClientRect.Call(a.main, uintptr(unsafe.Pointer(&r)))
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w <= 0 || h <= 0 {
		return
	}
	dpi := a.dpi
	if dpi < 96 {
		dpi = 96
	}
	sc := func(v int32) int32 { return v * dpi / 96 }
	pad, gap := sc(22), sc(12)
	rowH, labelH := sc(40), sc(22)
	positions := make([]childLayout, 0, 17)
	add := func(hwnd uintptr, x, y, cw, ch int32) {
		if hwnd != 0 {
			positions = append(positions, childLayout{hwnd: hwnd, x: x, y: y, w: cw, h: ch})
		}
	}

	// Header controls remain vertically centered in a full-width navy band.
	add(a.header, pad, sc(13), sc(330), sc(34))
	add(a.lang, max32(pad, w-pad-sc(142)), sc(13), sc(128), sc(36))

	// Query card: label and single-line input, aligned with both action buttons.
	y := sc(70)
	add(a.queryLabel, pad, y, max32(180, w-2*pad), labelH)
	y += labelH + sc(4)
	cancelW, searchW := sc(104), sc(112)
	queryW := w - 2*pad - 2*gap - cancelW - searchW
	if queryW < sc(160) {
		queryW = sc(160)
	}
	add(a.query, pad, y, queryW, rowH)
	add(a.searchBtn, pad+queryW+gap, y, searchW, rowH)
	add(a.cancelBtn, pad+queryW+gap+searchW+gap, y, cancelW, rowH)

	// Four filter controls share one grid and are moved as a single batch.
	y = sc(148)
	labelControls := []uintptr{a.rangeLabel, a.kindLabel, a.includeLabel, a.excludeLabel}
	editControls := []uintptr{a.drive, a.kind, a.include, a.exclude}
	avail := w - 2*pad - 3*gap
	if avail < sc(4*130) {
		avail = sc(4 * 130)
	}
	colW := avail / 4
	for i := 0; i < 4; i++ {
		x := pad + int32(i)*(colW+gap)
		cw := colW
		if i == 3 {
			cw = w - pad - x
		}
		add(labelControls[i], x, y, cw, labelH)
		add(editControls[i], x, y+labelH+sc(4), cw, rowH)
	}

	// Status and owner-data list consume the remaining vertical space.
	y = sc(232)
	add(a.status, pad+sc(2), y, max32(200, w-2*pad), sc(26))
	y += sc(29)
	listH := h - y - sc(18)
	if listH < sc(100) {
		listH = sc(100)
	}
	add(a.list, pad, y, max32(300, w-2*pad), listH)

	// Batch all child moves. Repositioning every control independently with
	// repaint=true caused repeated synchronous paints during interactive resize.
	hdwp, _, _ := procBeginDeferWindowPos.Call(uintptr(len(positions)))
	deferred := hdwp != 0
	if deferred {
		for _, p := range positions {
			hdwp, _, _ = procDeferWindowPos.Call(hdwp, p.hwnd, 0,
				uintptr(p.x), uintptr(p.y), uintptr(p.w), uintptr(p.h), uintptr(SWP_NOZORDER|SWP_NOACTIVATE))
			if hdwp == 0 {
				deferred = false
				break
			}
		}
	}
	if deferred {
		procEndDeferWindowPos.Call(hdwp)
	} else {
		// Rare fallback if USER32 cannot allocate a deferred-position batch.
		for _, p := range positions {
			procMoveWindow.Call(p.hwnd, uintptr(p.x), uintptr(p.y), uintptr(p.w), uintptr(p.h), 1)
		}
	}

	// Updating the path column on every mouse-move made ListView recalculate and
	// redraw over and over. Keep its previous width during dragging, then fit once.
	pathWidth := max32(sc(320), w-pad*2-sc(260+110+110+160+160+160)-sc(15))
	if !a.inSizeMove && pathWidth != a.pathColWidth {
		send(a.list, LVM_SETCOLUMNWIDTH, 6, uintptr(pathWidth))
		a.pathColWidth = pathWidth
	}
}

func rgb(r, g, b uintptr) uintptr { return r | (g << 8) | (b << 16) }

func (a *App) paintBackground(hdc uintptr) {
	var client RECT
	procGetClientRect.Call(a.main, uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), a.bgBrush)
	dpi := a.dpi
	if dpi < 96 {
		dpi = 96
	}
	sc := func(v int32) int32 { return v * dpi / 96 }
	header := RECT{Left: 0, Top: 0, Right: client.Right, Bottom: sc(62)}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&header)), a.headerBrush)
	// White rounded panels separate the main query and the filter controls from the canvas.
	a.paintPanel(hdc, RECT{Left: sc(12), Top: sc(65), Right: client.Right - sc(12), Bottom: sc(141)})
	a.paintPanel(hdc, RECT{Left: sc(12), Top: sc(145), Right: client.Right - sc(12), Bottom: sc(225)})
}

func (a *App) paintPanel(hdc uintptr, r RECT) {
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	dpi := a.dpi
	if dpi < 96 {
		dpi = 96
	}
	oldPen, _, _ := procSelectObject.Call(hdc, a.cardPen)
	oldBrush, _, _ := procSelectObject.Call(hdc, a.cardBrush)
	radius := uintptr(10 * dpi / 96)
	procRoundRect.Call(hdc, uintptr(unsafe.Pointer(&r)), radius, radius)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
}

func (a *App) resetCardPen() {
	if a.cardPen != 0 {
		procDeleteObject.Call(a.cardPen)
	}
	dpi := a.dpi
	if dpi < 96 {
		dpi = 96
	}
	pen, _, _ := procCreatePen.Call(0, uintptr(max32(1, dpi/96)), rgb(226, 232, 240))
	a.cardPen = pen
}

func (a *App) drawButton(item *DRAWITEMSTRUCT) {
	if item == nil || item.HDC == 0 {
		return
	}
	dpi := a.dpi
	if dpi < 96 {
		dpi = 96
	}
	isSearch := uintptr(item.CtlID) == 1003
	disabled := item.ItemState&0x00000004 != 0 // ODS_DISABLED
	pressed := item.ItemState&0x00000001 != 0  // ODS_SELECTED
	var fill, text uintptr
	if isSearch {
		fill, text = rgb(37, 99, 235), rgb(255, 255, 255)
		if pressed {
			fill = rgb(29, 78, 216)
		}
		if disabled {
			fill, text = rgb(190, 203, 224), rgb(255, 255, 255)
		}
	} else {
		fill, text = rgb(235, 240, 247), rgb(51, 65, 85)
		if pressed {
			fill = rgb(220, 228, 239)
		}
		if disabled {
			fill, text = rgb(242, 244, 248), rgb(156, 163, 175)
		}
	}
	brush, _, _ := procCreateSolidBrush.Call(fill)
	pen, _, _ := procCreatePen.Call(0, 1, fill)
	oldPen, _, _ := procSelectObject.Call(item.HDC, pen)
	oldBrush, _, _ := procSelectObject.Call(item.HDC, brush)
	r := item.RcItem
	inset := int32(1 * dpi / 96)
	r.Left += inset
	r.Top += inset
	r.Right -= inset
	r.Bottom -= inset
	radius := uintptr(8 * dpi / 96)
	procRoundRect.Call(item.HDC, uintptr(unsafe.Pointer(&r)), radius, radius)
	procSelectObject.Call(item.HDC, oldBrush)
	procSelectObject.Call(item.HDC, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
	procSetBkMode.Call(item.HDC, 1)
	procSetTextColor.Call(item.HDC, text)
	label := wptr(getText(item.HwndItem))
	textRect := item.RcItem
	flags := uintptr(0x00000001 | 0x00000004 | 0x00000020) // DT_CENTER | DT_VCENTER | DT_SINGLELINE
	procDrawTextW.Call(item.HDC, uintptr(unsafe.Pointer(label)), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), flags)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
func getClassName(hwnd uintptr) string {
	p := user32.NewProc("GetClassNameW")
	b := make([]uint16, 128)
	n, _, _ := p.Call(hwnd, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b[:n])
}
func (a *App) refreshListCount() {
	a.mu.RLock()
	n := len(a.results)
	a.mu.RUnlock()
	old := a.lastShownCount
	send(a.list, LVM_SETITEMCOUNT, uintptr(n), LVSICF_NOINVALIDATEALL|LVSICF_NOSCROLL)
	a.lastShownCount = n
	if n < old {
		// A new search cleared the old result set. Invalidate once for the clear,
		// but do not repaint the entire list on every incremental batch.
		procInvalidateRect.Call(a.list, 0, 0)
		return
	}
	if n <= old || n == 0 {
		return
	}
	top := int(send(a.list, LVM_GETTOPINDEX, 0, 0))
	perPage := int(send(a.list, LVM_GETCOUNTPERPAGE, 0, 0))
	if perPage < 1 {
		perPage = 32
	}
	first := old
	if first < top {
		first = top
	}
	last := top + perPage - 1
	if last >= n {
		last = n - 1
	}
	if first <= last {
		send(a.list, LVM_REDRAWITEMS, uintptr(first), uintptr(last))
	}
}

func (a *App) cellText(row, col int) string {
	a.mu.RLock()
	if row < 0 || row >= len(a.results) {
		a.mu.RUnlock()
		return ""
	}
	r := a.results[row]
	a.mu.RUnlock()
	if col >= 2 && col <= 5 && !r.MetaLoaded && !(col == 2 && r.IsDir) {
		a.ensureMetadata(row, r.Path)
		return "…"
	}
	switch col {
	case 0:
		return r.Name
	case 1:
		return r.Type
	case 2:
		if r.IsDir {
			return "—"
		}
		if !r.MetaLoaded {
			return "…"
		}
		return humanSize(r.Size)
	case 3:
		return timeFmt(r.Created)
	case 4:
		return timeFmt(r.Modified)
	case 5:
		return timeFmt(r.Accessed)
	case 6:
		return r.Path
	}
	return ""
}
func (a *App) ensureMetadata(row int, path string) {
	a.mu.Lock()
	if row < 0 || row >= len(a.results) || a.results[row].Path != path || a.results[row].MetaLoaded || a.results[row].MetaLoading {
		a.mu.Unlock()
		return
	}
	a.results[row].MetaLoading = true
	a.mu.Unlock()
	go func() {
		var meta FileResult
		loadMetadata(path, &meta)
		a.mu.Lock()
		updatedRow := -1
		for i := range a.results {
			if a.results[i].Path == path {
				a.results[i].Size = meta.Size
				a.results[i].Created = meta.Created
				a.results[i].Modified = meta.Modified
				a.results[i].Accessed = meta.Accessed
				a.results[i].MetaLoaded = true
				a.results[i].MetaLoading = false
				updatedRow = i
				break
			}
		}
		a.mu.Unlock()
		if updatedRow >= 0 {
			procPostMessageW.Call(a.main, WM_APP_ROW_REDRAW, uintptr(updatedRow), 0)
		}
	}()
}

func (a *App) updateSortHeaders() {
	a.mu.RLock()
	col, asc := a.sortCol, a.sortAsc
	a.mu.RUnlock()
	cols := []string{a.localize("名称", "Name"), a.localize("类型", "Type"), a.localize("大小", "Size"), a.localize("创建时间", "Created"), a.localize("最后写入", "Modified"), a.localize("上次访问", "Accessed"), a.localize("完整路径", "Full path")}
	if col >= 0 && col < len(cols) {
		if asc {
			cols[col] += "  ↑"
		} else {
			cols[col] += "  ↓"
		}
	}
	for i, c := range cols {
		p := wptr(c)
		column := LVCOLUMNW{Mask: LVCF_TEXT, Text: uintptr(unsafe.Pointer(p)), CchTextMax: int32(len(utf16.Encode([]rune(c))))}
		send(a.list, LVM_SETCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&column)))
	}
}

func (a *App) changeLanguage() {
	// Preserve both selections: changing language must not silently reset the
	// user's chosen volume or file-type filter.
	driveSel := int(send(a.drive, CB_GETCURSEL, 0, 0))
	kindSel := int(send(a.kind, CB_GETCURSEL, 0, 0))
	sel := int(send(a.lang, CB_GETCURSEL, 0, 0))
	a.english = sel == 1
	callText(a.queryLabel, a.localize("文件名或路径", "File name or path"))
	callText(a.rangeLabel, a.localize("搜索范围", "Search in"))
	callText(a.kindLabel, a.localize("显示类型", "Show type"))
	callText(a.includeLabel, a.localize("自定义扩展名", "Include extensions"))
	callText(a.excludeLabel, a.localize("排除扩展名", "Exclude extensions"))
	callText(a.searchBtn, a.localize("搜索", "Search"))
	callText(a.cancelBtn, a.localize("取消", "Cancel"))
	send(a.kind, CB_RESETCONTENT, 0, 0)
	items := []string{a.localize("全部文件", "All files"), a.localize("仅文件", "Files only"), a.localize("仅文件夹", "Folders only"), a.localize("图片", "Images"), a.localize("视频", "Videos"), a.localize("音频", "Audio"), a.localize("文档", "Documents"), a.localize("压缩包", "Archives"), a.localize("程序/安装包", "Programs/installers")}
	a.addCombo(a.kind, items)
	if kindSel < 0 || kindSel >= len(items) {
		kindSel = 0
	}
	send(a.kind, CB_SETCURSEL, uintptr(kindSel), 0)
	send(a.drive, CB_RESETCONTENT, 0, 0)
	a.mu.RLock()
	drives := append([]string(nil), a.drives...)
	loading := a.drivesLoading
	searching := a.searching
	a.mu.RUnlock()
	vols := []string{a.localize("全部 NTFS 卷", "All NTFS volumes")}
	if loading {
		vols = []string{a.localize("正在识别 NTFS 卷…", "Detecting NTFS volumes…")}
	} else if len(drives) > 0 {
		vols = append(vols, drives...)
	} else {
		vols = []string{a.localize("未找到 NTFS 卷", "No NTFS volumes found")}
	}
	a.addCombo(a.drive, vols)
	if driveSel < 0 || driveSel >= len(vols) {
		driveSel = 0
	}
	send(a.drive, CB_SETCURSEL, uintptr(driveSel), 0)
	a.updateSortHeaders()
	a.resetFonts()
	a.layout()
	if !searching {
		callText(a.status, a.localize("就绪", "Ready"))
	}
}

const LVM_SETCOLUMNW = LVM_FIRST + 96

func (a *App) startSearch() {
	a.mu.RLock()
	sorting := a.sorting
	a.mu.RUnlock()
	if sorting {
		callText(a.status, a.localize("请等待当前排序完成。", "Wait for sorting to finish."))
		return
	}
	query := strings.TrimSpace(getText(a.query))
	if query == "" {
		callText(a.status, a.localize("请输入文件名或路径。", "Enter a file name or path."))
		procSetFocus.Call(a.query)
		return
	}
	a.cancelSearchNow()
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.searchCancel = cancel
	a.searching = true
	a.resultsDone = false
	a.resultsDirty = false
	a.results = nil
	a.statusText = a.localize("准备搜索…", "Preparing search…")
	a.generation++
	gen := a.generation
	a.mu.Unlock()
	procEnableWindow.Call(a.searchBtn, 0)
	procEnableWindow.Call(a.cancelBtn, 1)
	a.refreshListCount()
	callText(a.status, a.localize("准备搜索…", "Preparing search…"))
	vols := a.selectedVolumes()
	kindSel := int(send(a.kind, CB_GETCURSEL, 0, 0))
	include := parseExts(getText(a.include))
	exclude := parseExts(getText(a.exclude))
	go func() {
		defer procPostMessageW.Call(a.main, WM_APP_SEARCH_DONE, uintptr(gen), 0)
		if len(vols) == 0 {
			a.setStatus(gen, a.localize("没有找到可搜索的 NTFS 卷。", "No searchable NTFS volumes found."))
			return
		}
		var total int64
		var errs []string
		for _, vol := range vols {
			if ctx.Err() != nil {
				break
			}
			idx, err := a.getIndex(ctx, vol, gen)
			if err != nil {
				errs = append(errs, vol+": "+err.Error())
				continue
			}
			count, err := a.runIndexSearch(ctx, idx, vol, query, kindSel, include, exclude, gen)
			total += count
			if err != nil && ctx.Err() == nil {
				errs = append(errs, err.Error())
			}
		}
		if ctx.Err() != nil {
			a.setStatus(gen, a.localize("已取消。", "Cancelled."))
			return
		}
		if len(errs) > 0 {
			a.setStatus(gen, fmt.Sprintf("%s %d %s | %s", a.localize("完成：", "Done:"), total, a.localize("条结果", "results"), strings.Join(errs, "; ")))
		} else {
			a.setStatus(gen, fmt.Sprintf("%s %d %s", a.localize("完成：", "Done:"), total, a.localize("条结果", "results")))
		}
	}()
}
func (a *App) cancelSearchNow() {
	a.mu.Lock()
	c := a.searchCancel
	if c != nil {
		c()
		a.searchCancel = nil
	}
	a.mu.Unlock()
}
func (a *App) setStatus(gen int64, s string) {
	a.mu.Lock()
	if gen == a.generation {
		a.statusText = s
	}
	a.mu.Unlock()
	procPostMessageW.Call(a.main, WM_APP_STATUS, 0, 0)
}
func (a *App) selectedVolumes() []string {
	a.mu.RLock()
	drives := append([]string(nil), a.drives...)
	loading := a.drivesLoading
	a.mu.RUnlock()
	if loading || len(drives) == 0 {
		return nil
	}
	sel := int(send(a.drive, CB_GETCURSEL, 0, 0))
	if sel <= 0 {
		return drives
	}
	if sel-1 < len(drives) {
		return []string{drives[sel-1]}
	}
	return nil
}
func (a *App) enumerateNTFSVolumes() []string {
	mask, _, _ := procGetLogicalDrives.Call()
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		drive := string(rune('A'+i)) + ":"
		root := drive + "\\"
		rootPtr := wptr(root)
		driveType, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
		if driveType == 4 || driveType == 5 { // skip remote and optical drives (network enumeration can block UI)
			continue
		}
		fs := make([]uint16, 64)
		serial := uint32(0)
		r, _, _ := procGetVolumeInformationW.Call(uintptr(unsafe.Pointer(wptr(root))), 0, 0, uintptr(unsafe.Pointer(&serial)), 0, 0, uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs)))
		if r != 0 && strings.EqualFold(syscall.UTF16ToString(fs), "NTFS") {
			out = append(out, drive)
		}
	}
	return out
}
func (a *App) getIndex(ctx context.Context, drive string, gen int64) (*VolumeIndex, error) {
	// Reuse the in-process index first; subsequent searches do not rescan the disk.
	a.mu.RLock()
	idx := a.indices[drive]
	a.mu.RUnlock()
	if idx != nil {
		return idx, nil
	}
	serial, jid, next, cacheOK, handle, err := openVolumeState(drive)
	if err != nil {
		return nil, err
	}
	defer procCloseHandle.Call(handle)
	cachePath := a.cachePath(drive)
	if cacheOK {
		if cached, cachedNext, ok := readCache(cachePath, serial, jid, next); ok {
			var entries []Entry
			appliedNext := cachedNext
			if cachedNext == next {
				entries = cached
			} else if cachedNext < next {
				a.setStatus(gen, fmt.Sprintf("%s %s", a.localize("正在更新增量索引：", "Updating index changes:"), drive))
				entries, appliedNext, err = updateFromJournal(ctx, handle, jid, cachedNext, next, cached)
				if err != nil {
					entries = nil // a journal gap/rollover means a clean rebuild is safer
				}
			}
			if entries != nil {
				idx = buildIndex(entries, serial, jid, appliedNext)
				a.mu.Lock()
				a.indices[drive] = idx
				a.mu.Unlock()
				// Do not rewrite a huge index for every small journal change. Keep the
				// previous checkpoint and replay deltas on the next launch; checkpoint
				// periodically once the journal distance is substantial.
				if appliedNext-cachedNext >= 64*1024*1024 {
					go func(snapshot *VolumeIndex) { _ = writeCache(cachePath, snapshot) }(idx)
				}
				a.setStatus(gen, fmt.Sprintf("%s %s", a.localize("已加载磁盘索引：", "Loaded disk index:"), drive))
				return idx, nil
			}
		}
	}
	a.setStatus(gen, fmt.Sprintf("%s %s", a.localize("首次建立磁盘索引：", "Building first disk index:"), drive))
	initialNext := next
	entries, err := enumerateUSN(ctx, handle, func(n int64) {
		a.setStatus(gen, fmt.Sprintf("%s %s — %d %s", a.localize("正在扫描", "Scanning"), drive, n, a.localize("条记录", "records")))
	})
	if err != nil {
		return nil, err
	}
	cacheStable := false
	indexNext := initialNext
	serial2, jid2, next2, _, handle2, err2 := openVolumeState(drive)
	if handle2 != 0 {
		procCloseHandle.Call(handle2)
	}
	if cacheOK && err2 == nil && serial2 == serial && jid2 == jid && next2 >= initialNext {
		if next2 == initialNext {
			cacheStable = true
		} else {
			// Reconcile file changes that occurred while enumerating MFT, then cache
			// a consistent snapshot instead of throwing away the index on busy disks.
			if reconciled, reconciledNext, deltaErr := updateFromJournal(ctx, handle, jid, initialNext, next2, entries); deltaErr == nil {
				entries = reconciled
				indexNext = reconciledNext
				cacheStable = true
			}
		}
	}
	idx = buildIndex(entries, serial, jid, indexNext)
	a.mu.Lock()
	a.indices[drive] = idx
	a.mu.Unlock()
	if cacheStable {
		go func(snapshot *VolumeIndex) { _ = writeCache(cachePath, snapshot) }(idx)
	}
	return idx, nil
}

// Apply NTFS USN Journal deltas to a cached index. If the journal has wrapped,
// changed ID, or cannot be read consistently, the caller falls back to a full MFT scan.
func updateFromJournal(ctx context.Context, h uintptr, journalID uint64, startUSN, targetUSN int64, entries []Entry) ([]Entry, int64, error) {
	if startUSN >= targetUSN {
		return entries, targetUSN, nil
	}
	var jd USN_JOURNAL_DATA
	var returned uint32
	ok, _, callErr := procDeviceIoControl.Call(h, FSCTL_QUERY_USN_JOURNAL, 0, 0,
		uintptr(unsafe.Pointer(&jd)), uintptr(unsafe.Sizeof(jd)), uintptr(unsafe.Pointer(&returned)), 0)
	if ok == 0 {
		return nil, startUSN, fmt.Errorf("query USN journal: %v", callErr)
	}
	if jd.JournalID != journalID || startUSN < jd.LowestValidUSN || startUSN > jd.NextUSN {
		return nil, startUSN, fmt.Errorf("cached journal range is no longer valid")
	}
	// A map of FRN to slice index uses substantially less memory than copying every
	// Entry into a second map, especially on volumes with millions of files.
	byID := make(map[uint64]int, len(entries))
	for i, e := range entries {
		byID[e.ID] = i
	}
	const reasonFileDelete = uint32(0x00000200)
	const reasonRenameOld = uint32(0x00001000)
	buf := make([]byte, 1024*1024)
	cursor := startUSN
	for cursor < targetUSN {
		if ctx.Err() != nil {
			return nil, cursor, ctx.Err()
		}
		in := READ_USN_JOURNAL_DATA{StartUSN: cursor, ReasonMask: 0xFFFFFFFF, JournalID: journalID}
		var n uint32
		ret, _, callErr := procDeviceIoControl.Call(h, FSCTL_READ_USN_JOURNAL,
			uintptr(unsafe.Pointer(&in)), uintptr(unsafe.Sizeof(in)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), 0)
		if ret == 0 {
			if cursor >= targetUSN && (n == 0) {
				break
			}
			return nil, cursor, fmt.Errorf("read USN journal: %v", callErr)
		}
		if n < 8 {
			if cursor >= targetUSN {
				break
			}
			return nil, cursor, fmt.Errorf("USN journal returned an empty range")
		}
		next := int64(binary.LittleEndian.Uint64(buf[:8]))
		pos := 8
		for pos+60 <= int(n) {
			length := int(binary.LittleEndian.Uint32(buf[pos:]))
			if length < 60 || pos+length > int(n) {
				break
			}
			if binary.LittleEndian.Uint16(buf[pos+4:]) == 2 {
				id := binary.LittleEndian.Uint64(buf[pos+8:])
				parent := binary.LittleEndian.Uint64(buf[pos+16:])
				reason := binary.LittleEndian.Uint32(buf[pos+40:])
				attrs := binary.LittleEndian.Uint32(buf[pos+52:])
				nameLen := int(binary.LittleEndian.Uint16(buf[pos+56:]))
				nameOff := int(binary.LittleEndian.Uint16(buf[pos+58:]))
				if reason&reasonFileDelete != 0 {
					if at, exists := byID[id]; exists {
						entries[at].Attr = 0xFFFFFFFF
						entries[at].Name = ""
						delete(byID, id)
					}
				} else if reason&reasonRenameOld == 0 && nameLen > 0 && nameOff+nameLen <= length {
					u := make([]uint16, nameLen/2)
					for j := range u {
						u[j] = binary.LittleEndian.Uint16(buf[pos+nameOff+j*2:])
					}
					updated := Entry{ID: id, Parent: parent, Attr: attrs, Name: string(utf16.Decode(u))}
					if at, exists := byID[id]; exists {
						entries[at] = updated
					} else {
						byID[id] = len(entries)
						entries = append(entries, updated)
					}
				}
			}
			pos += length
		}
		if next <= cursor {
			return nil, cursor, fmt.Errorf("USN journal cursor did not advance")
		}
		cursor = next
	}
	write := 0
	for _, e := range entries {
		if e.Attr == 0xFFFFFFFF {
			continue
		}
		entries[write] = e
		write++
	}
	entries = entries[:write]
	return entries, cursor, nil
}

func (a *App) cachePath(drive string) string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "SwiftSearchPlus", "index-"+strings.TrimSuffix(drive, ":")+".dat")
}
func (a *App) runIndexSearch(ctx context.Context, idx *VolumeIndex, drive, query string, kindSel int, include, exclude map[string]bool, gen int64) (int64, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	hasPath := strings.ContainsAny(q, "\\/")
	entryCount := len(idx.Entries)
	if entryCount == 0 {
		return 0, nil
	}

	// Matching and path construction are CPU-bound once the NTFS index is in
	// memory. Scan disjoint ranges on a bounded worker pool; each worker owns its
	// directory-path cache and result batch, so no map needs to be shared.
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	if entryCount < workers*12000 {
		workers = maxInt(1, entryCount/12000)
	}
	if workers < 1 {
		workers = 1
	}

	var processed atomic.Int64
	var lastStatus atomic.Int64
	type workerResult struct {
		count int64
	}
	done := make(chan workerResult, workers)
	chunk := (entryCount + workers - 1) / workers
	for wi := 0; wi < workers; wi++ {
		start := wi * chunk
		end := start + chunk
		if end > entryCount {
			end = entryCount
		}
		if start >= end {
			done <- workerResult{}
			continue
		}
		go func(start, end int) {
			pathCache := make(map[uint64]string, 2048)
			batch := make([]FileResult, 0, 512)
			var count int64
			lastFlush := time.Now()
			localProcessed := 0
			flush := func() {
				if len(batch) == 0 {
					return
				}
				count += int64(a.appendResults(gen, batch))
				batch = batch[:0]
				lastFlush = time.Now()
			}
			for i := start; i < end; i++ {
				if localProcessed&0x1fff == 0 {
					if ctx.Err() != nil {
						break
					}
					a.mu.RLock()
					full := len(a.results) >= MAX_RESULTS
					a.mu.RUnlock()
					if full {
						break
					}
					if localProcessed > 0 {
						processed.Add(int64(localProcessed))
						localProcessed = 0
						now := time.Now().UnixMilli()
						prev := lastStatus.Load()
						if now-prev >= 500 && lastStatus.CompareAndSwap(prev, now) {
							doneN := processed.Load()
							a.setStatus(gen, fmt.Sprintf("%s %s — %d / %d", a.localize("正在匹配", "Matching"), drive, doneN, entryCount))
						}
					}
				}
				localProcessed++
				e := idx.Entries[i]
				isDir := e.Attr&FILE_ATTRIBUTE_DIRECTORY != 0
				if (kindSel == 1 || kindSel >= 3) && isDir {
					continue
				}
				if kindSel == 2 && !isDir {
					continue
				}
				ext := filepath.Ext(e.LowerName)
				if !isDir && !typeCategoryMatchesExt(kindSel, ext) {
					continue
				}
				if len(include) > 0 && !isDir && !include[ext] {
					continue
				}
				if exclude[ext] {
					continue
				}
				var fullPath string
				if hasPath {
					fullPath = buildFullPath(drive, e, idx.Dirs, pathCache)
					if !matchLower(q, strings.ToLower(fullPath)) {
						continue
					}
				} else if !matchLower(q, e.LowerName) {
					continue
				}
				if fullPath == "" {
					fullPath = buildFullPath(drive, e, idx.Dirs, pathCache)
				}
				typ := strings.TrimPrefix(ext, ".")
				if isDir {
					typ = a.localize("文件夹", "Folder")
				} else if typ == "" {
					typ = a.localize("文件", "File")
				}
				batch = append(batch, FileResult{Name: e.Name, Type: typ, Path: fullPath, IsDir: isDir})
				if len(batch) >= 512 || time.Since(lastFlush) >= 350*time.Millisecond {
					flush()
				}
			}
			processed.Add(int64(localProcessed))
			flush()
			done <- workerResult{count: count}
		}(start, end)
	}

	var total int64
	for i := 0; i < workers; i++ {
		r := <-done
		total += r.count
	}
	if ctx.Err() != nil {
		return total, ctx.Err()
	}
	return total, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *App) appendResults(gen int64, batch []FileResult) int {
	added := 0
	a.mu.Lock()
	if gen == a.generation && len(a.results) < MAX_RESULTS {
		room := MAX_RESULTS - len(a.results)
		if len(batch) > room {
			batch = batch[:room]
		}
		a.results = append(a.results, batch...)
		added = len(batch)
		if added > 0 {
			a.resultsDirty = true
		}
	}
	a.mu.Unlock()
	return added
}

func (a *App) sortByColumn(col int) {
	a.mu.Lock()
	if a.searching || a.sorting {
		a.mu.Unlock()
		a.setStatus(a.generation, a.localize("搜索或排序完成后再操作。", "Wait for search/sort to finish."))
		return
	}
	if col == a.sortCol {
		a.sortAsc = !a.sortAsc
	} else {
		a.sortCol = col
		a.sortAsc = true
	}
	asc, c := a.sortAsc, col
	a.sorting = true
	a.mu.Unlock()
	a.updateSortHeaders()
	callText(a.status, a.localize("正在排序…", "Sorting…"))
	go func() {
		a.mu.RLock()
		src := append([]FileResult(nil), a.results...)
		a.mu.RUnlock()
		// Sorting by size/time requires metadata for every row. Load it in a bounded
		// worker pool so the window remains responsive and the data sort is meaningful.
		if c >= 2 && c <= 5 {
			jobs := make(chan int, 256)
			var wg sync.WaitGroup
			workers := runtime.NumCPU()
			if workers > 8 {
				workers = 8
			}
			if workers < 2 {
				workers = 2
			}
			for n := 0; n < workers; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range jobs {
						if !src[i].MetaLoaded {
							loadMetadata(src[i].Path, &src[i])
							src[i].MetaLoaded = true
							src[i].MetaLoading = false
						}
					}
				}()
			}
			for i := range src {
				if !src[i].MetaLoaded {
					jobs <- i
				}
			}
			close(jobs)
			wg.Wait()
		}
		sort.SliceStable(src, func(i, j int) bool {
			x, y := src[i], src[j]
			cmp := 0
			switch c {
			case 0:
				cmp = strings.Compare(x.NameKey, y.NameKey)
			case 1:
				cmp = strings.Compare(strings.ToLower(x.Type), strings.ToLower(y.Type))
			case 2:
				if x.Size < y.Size {
					cmp = -1
				} else if x.Size > y.Size {
					cmp = 1
				}
			case 3:
				if x.Created.Before(y.Created) {
					cmp = -1
				} else if x.Created.After(y.Created) {
					cmp = 1
				}
			case 4:
				if x.Modified.Before(y.Modified) {
					cmp = -1
				} else if x.Modified.After(y.Modified) {
					cmp = 1
				}
			case 5:
				if x.Accessed.Before(y.Accessed) {
					cmp = -1
				} else if x.Accessed.After(y.Accessed) {
					cmp = 1
				}
			case 6:
				cmp = strings.Compare(strings.ToLower(x.Path), strings.ToLower(y.Path))
			}
			if asc {
				return cmp < 0
			}
			return cmp > 0
		})
		a.mu.Lock()
		a.pendingSorted = src
		a.mu.Unlock()
		procPostMessageW.Call(a.main, WM_APP_SORT_DONE, 0, 0)
	}()
}

func (a *App) openResult(row int, selectInExplorer bool) {
	a.mu.RLock()
	if row < 0 || row >= len(a.results) {
		a.mu.RUnlock()
		return
	}
	p := a.results[row].Path
	a.mu.RUnlock()
	if selectInExplorer {
		a.shell("open", "explorer.exe", `/select,"`+p+`"`)
	} else {
		a.shell("open", p, "")
	}
}
func (a *App) shell(verb, file, args string) {
	v, f, ar := wptr(verb), wptr(file), wptr(args)
	procShellExecuteW.Call(a.main, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(f)), uintptr(unsafe.Pointer(ar)), 0, SW_SHOWNORMAL)
}
func (a *App) showContextMenu() {
	var screenPt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&screenPt)))
	clientPt := screenPt
	// Translate screen point into list-view client coordinates, then select the row under the pointer.
	proc := user32.NewProc("ScreenToClient")
	proc.Call(a.list, uintptr(unsafe.Pointer(&clientPt)))
	hit := LVHITTESTINFO{Pt: clientPt, IItem: -1}
	send(a.list, LVM_HITTEST, 0, uintptr(unsafe.Pointer(&hit)))
	if hit.IItem >= 0 {
		const LVM_SETITEMSTATE = LVM_FIRST + 43
		const LVIS_FOCUSED = 0x0001
		const LVIS_SELECTED = 0x0002
		clear := LVITEMW{State: 0, StateMask: LVIS_FOCUSED | LVIS_SELECTED}
		send(a.list, LVM_SETITEMSTATE, ^uintptr(0), uintptr(unsafe.Pointer(&clear)))
		a.selectRow(int(hit.IItem))
	}
	menu, _, _ := procCreatePopupMenu.Call()
	labels := []string{a.localize("打开", "Open"), a.localize("在资源管理器中定位", "Show in Explorer"), a.localize("复制完整路径", "Copy full path"), a.localize("复制文件名", "Copy file name"), a.localize("复制选中行", "Copy selected row")}
	for i, s := range labels {
		p := wptr(s)
		procAppendMenuW.Call(menu, MF_STRING, uintptr(5001+i), uintptr(unsafe.Pointer(p)))
	}
	// TrackPopupMenu expects screen coordinates.
	cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_RIGHTBUTTON, uintptr(screenPt.X), uintptr(screenPt.Y), 0, a.main, 0)
	procDestroyMenu.Call(menu)
	switch cmd {
	case 5001:
		a.openSelected(false)
	case 5002:
		a.openSelected(true)
	case 5003:
		a.copySelected(0)
	case 5004:
		a.copySelected(1)
	case 5005:
		a.copySelected(2)
	}
}
func (a *App) selectRow(row int) {
	const LVM_SETITEMSTATE = LVM_FIRST + 43
	const LVIS_FOCUSED = 0x0001
	const LVIS_SELECTED = 0x0002
	st := LVITEMW{State: LVIS_FOCUSED | LVIS_SELECTED, StateMask: LVIS_FOCUSED | LVIS_SELECTED}
	send(a.list, LVM_SETITEMSTATE, uintptr(row), uintptr(unsafe.Pointer(&st)))
}
func (a *App) selectedRow() int {
	v := send(a.list, LVM_GETNEXTITEM, ^uintptr(0), LVNI_SELECTED)
	if int(v) >= 0 && v < 0x7fffffff {
		return int(v)
	}
	return -1
}
func (a *App) openSelected(explorer bool) { a.openResult(a.selectedRow(), explorer) }
func (a *App) copySelected(mode int) {
	row := a.selectedRow()
	a.mu.RLock()
	if row < 0 || row >= len(a.results) {
		a.mu.RUnlock()
		return
	}
	r := a.results[row]
	a.mu.RUnlock()
	s := r.Path
	if mode == 1 {
		s = r.Name
	}
	if mode == 2 {
		s = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", r.Name, r.Type, humanSize(r.Size), timeFmt(r.Created), timeFmt(r.Modified), timeFmt(r.Accessed), r.Path)
	}
	a.copyText(s)
}
func (a *App) copyText(s string) {
	r, _, _ := procOpenClipboard.Call(a.main)
	if r == 0 {
		return
	}
	procEmptyClipboard.Call()
	u, _ := syscall.UTF16FromString(s)
	size := uintptr(len(u) * 2)
	h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	p, _, _ := procGlobalLock.Call(h)
	if p != 0 {
		copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(u)), u)
		procGlobalUnlock.Call(h)
		procSetClipboardData.Call(CF_UNICODETEXT, h)
	}
	procCloseClipboard.Call()
}

func openVolumeState(drive string) (uint32, uint64, int64, bool, uintptr, error) {
	path := "\\\\.\\" + drive
	p := wptr(path)
	h, _, e := procCreateFileW.Call(uintptr(unsafe.Pointer(p)), GENERIC_READ, FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE, 0, OPEN_EXISTING, 0, 0)
	if h == INVALID_HANDLE_VALUE {
		return 0, 0, 0, false, 0, fmt.Errorf("open volume failed: %v", e)
	}
	root := drive + "\\"
	var serial uint32
	fs := make([]uint16, 64)
	r, _, _ := procGetVolumeInformationW.Call(uintptr(unsafe.Pointer(wptr(root))), 0, 0, uintptr(unsafe.Pointer(&serial)), 0, 0, uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs)))
	if r == 0 || !strings.EqualFold(syscall.UTF16ToString(fs), "NTFS") {
		procCloseHandle.Call(h)
		return 0, 0, 0, false, 0, fmt.Errorf("not an NTFS volume")
	}
	var jd USN_JOURNAL_DATA
	var ret uint32
	r, _, _ = procDeviceIoControl.Call(h, FSCTL_QUERY_USN_JOURNAL, 0, 0, uintptr(unsafe.Pointer(&jd)), uintptr(unsafe.Sizeof(jd)), uintptr(unsafe.Pointer(&ret)), 0)
	if r == 0 {
		return serial, 0, 0, false, h, nil
	}
	return serial, jd.JournalID, jd.NextUSN, true, h, nil
}
func enumerateUSN(ctx context.Context, h uintptr, progress func(int64)) ([]Entry, error) {
	buf := make([]byte, 1024*1024)
	entries := make([]Entry, 0, 250000)
	var start uint64
	var seen int64
	var lastReported int64
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		in := USN_ENUM_DATA{StartFRN: start, LowUSN: 0, HighUSN: math.MaxInt64}
		var ret uint32
		r, _, e := procDeviceIoControl.Call(h, FSCTL_ENUM_USN_DATA, uintptr(unsafe.Pointer(&in)), uintptr(unsafe.Sizeof(in)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&ret)), 0)
		if r == 0 {
			if errno, ok := e.(syscall.Errno); ok && errno == 38 {
				break
			}
			if ret == 0 {
				break
			}
			return nil, fmt.Errorf("MFT enumeration failed: %v", e)
		}
		if ret < 8 {
			break
		}
		next := binary.LittleEndian.Uint64(buf[:8])
		pos := 8
		for pos+60 <= int(ret) {
			length := int(binary.LittleEndian.Uint32(buf[pos:]))
			if length < 60 || pos+length > int(ret) {
				break
			}
			major := binary.LittleEndian.Uint16(buf[pos+4:])
			if major == 2 {
				id := binary.LittleEndian.Uint64(buf[pos+8:])
				parent := binary.LittleEndian.Uint64(buf[pos+16:])
				attrs := binary.LittleEndian.Uint32(buf[pos+52:])
				nameLen := int(binary.LittleEndian.Uint16(buf[pos+56:]))
				nameOff := int(binary.LittleEndian.Uint16(buf[pos+58:]))
				if nameLen > 0 && nameOff+nameLen <= length {
					u := make([]uint16, nameLen/2)
					for j := range u {
						u[j] = binary.LittleEndian.Uint16(buf[pos+nameOff+j*2:])
					}
					name := string(utf16.Decode(u))
					entries = append(entries, Entry{ID: id, Parent: parent, Attr: attrs, Name: name})
					seen++
				}
			}
			pos += length
		}
		if seen-lastReported >= 50000 {
			progress(seen)
			lastReported = seen
		}
		if next == start || next == 0 {
			break
		}
		start = next
	}
	progress(seen)
	return entries, nil
}
func buildIndex(entries []Entry, serial uint32, jid uint64, next int64) *VolumeIndex {
	dirs := make(map[uint64]DirNode, len(entries)/8)
	for i := range entries {
		entries[i].LowerName = strings.ToLower(entries[i].Name)
		e := entries[i]
		if e.Attr&FILE_ATTRIBUTE_DIRECTORY != 0 {
			dirs[e.ID] = DirNode{Parent: e.Parent, Name: e.Name}
		}
	}
	return &VolumeIndex{Serial: serial, JournalID: jid, NextUSN: next, Entries: entries, Dirs: dirs}
}
func buildFullPath(drive string, e Entry, dirs map[uint64]DirNode, cache map[uint64]string) string {
	if p, ok := cache[e.ID]; ok {
		return p
	}
	parent := directoryPath(drive, e.Parent, dirs, cache, 0)
	if !strings.HasSuffix(parent, "\\") {
		parent += "\\"
	}
	p := parent + e.Name
	if e.Attr&FILE_ATTRIBUTE_DIRECTORY != 0 {
		cache[e.ID] = p
	}
	return p
}
func directoryPath(drive string, id uint64, dirs map[uint64]DirNode, cache map[uint64]string, depth int) string {
	root := strings.TrimRight(drive, "\\") + "\\"
	if id&0x0000FFFFFFFFFFFF == 5 || depth > 512 {
		return root
	}
	if p, ok := cache[id]; ok {
		if strings.HasSuffix(p, "\\") {
			return p
		}
		return p + "\\"
	}
	d, ok := dirs[id]
	if !ok {
		return root
	}
	base := directoryPath(drive, d.Parent, dirs, cache, depth+1)
	p := strings.TrimRight(base, "\\") + "\\" + d.Name + "\\"
	cache[id] = p
	return p
}
func joinReverse(parts []string) string {
	var b strings.Builder
	for i := len(parts) - 1; i >= 0; i-- {
		if i != len(parts)-1 {
			b.WriteByte('\\')
		}
		b.WriteString(parts[i])
	}
	return b.String()
}
func loadMetadata(path string, r *FileResult) {
	p := wptr(path)
	var d WIN32_FILE_ATTRIBUTE_DATA
	ok, _, _ := procGetFileAttributesExW.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		return
	}
	r.Size = int64(uint64(d.SizeHigh)<<32 | uint64(d.SizeLow))
	r.Created = filetime(d.Creation)
	r.Accessed = filetime(d.Access)
	r.Modified = filetime(d.Write)
}
func filetime(f FILETIME) time.Time {
	v := uint64(f.High)<<32 | uint64(f.Low)
	if v == 0 {
		return time.Time{}
	}
	const offset = 116444736000000000
	if v < offset {
		return time.Time{}
	}
	ns := int64(v-offset) * 100
	return time.Unix(0, ns)
}
func timeFmt(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
func humanSize(n int64) string {
	if n < 0 {
		return "—"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}
func matchName(q, s string) bool { return matchLower(strings.ToLower(q), strings.ToLower(s)) }
func matchLower(q, s string) bool {
	if q == "" {
		return true
	}
	if !strings.ContainsAny(q, "*?") {
		return strings.Contains(s, q)
	}
	return globMatch(q, s)
}

func globMatch(pattern, s string) bool {
	// Greedy wildcard matching is O(n) and allocates no per-file DP table.
	p, t := 0, 0
	star, retry := -1, 0
	for t < len(s) {
		if p < len(pattern) && (pattern[p] == '?' || pattern[p] == s[t]) {
			p++
			t++
			continue
		}
		if p < len(pattern) && pattern[p] == '*' {
			star = p
			p++
			retry = t
			continue
		}
		if star >= 0 {
			p = star + 1
			retry++
			t = retry
			continue
		}
		return false
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

func parseExts(s string) map[string]bool {
	m := map[string]bool{}
	for _, part := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ';' || r == ',' || r == ' ' || r == '\t' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			if !strings.HasPrefix(part, ".") {
				part = "." + part
			}
			m[part] = true
		}
	}
	return m
}

var categoryExtensions = map[int]map[string]bool{
	3: extSet(".jpg .jpeg .png .gif .bmp .webp .tif .tiff .ico .svg .heic .raw .cr2 .nef .arw .avif"),
	4: extSet(".mp4 .mkv .avi .mov .wmv .flv .webm .m4v .mpg .mpeg .ts .m2ts .3gp .vob .ogv"),
	5: extSet(".mp3 .flac .wav .aac .m4a .ogg .wma .opus .aiff .ape .mid .midi"),
	6: extSet(".txt .pdf .doc .docx .xls .xlsx .ppt .pptx .rtf .odt .csv .md .log .epub .odf .pages"),
	7: extSet(".zip .7z .rar .tar .gz .bz2 .xz .iso .cab .zst .tgz .lz4 .wim"),
	8: extSet(".exe .msi .msix .appx .bat .cmd .com .ps1 .dll .sys .scr .appinstaller"),
}

func extSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, e := range strings.Fields(s) {
		m[e] = true
	}
	return m
}
func typeCategoryMatches(sel int, name string) bool {
	return typeCategoryMatchesExt(sel, strings.ToLower(filepath.Ext(name)))
}
func typeCategoryMatchesExt(sel int, ext string) bool {
	if sel == 0 || sel == 1 || sel == 2 {
		return true
	}
	return categoryExtensions[sel][ext]
}

func readCache(path string, serial uint32, jid uint64, next int64) ([]Entry, int64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, false
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 2*1024*1024)
	hdr := make([]byte, 32)
	if _, err = readFull(r, hdr); err != nil || string(hdr[:4]) != "SSP2" {
		return nil, 0, false
	}
	cachedNext := int64(binary.LittleEndian.Uint64(hdr[16:]))
	if binary.LittleEndian.Uint32(hdr[4:]) != serial || binary.LittleEndian.Uint64(hdr[8:]) != jid || cachedNext > next {
		return nil, 0, false
	}
	count := binary.LittleEndian.Uint64(hdr[24:])
	if count > 100000000 {
		return nil, 0, false
	}
	entries := make([]Entry, 0, int(count))
	nameScratch := make([]byte, 65536)
	var fixed [24]byte
	for i := uint64(0); i < count; i++ {
		if _, err = readFull(r, fixed[:]); err != nil {
			return nil, 0, false
		}
		id := binary.LittleEndian.Uint64(fixed[0:])
		parent := binary.LittleEndian.Uint64(fixed[8:])
		attr := binary.LittleEndian.Uint32(fixed[16:])
		l := int(binary.LittleEndian.Uint32(fixed[20:]))
		if l < 0 || l > 65536 {
			return nil, 0, false
		}
		name := nameScratch[:l]
		if _, err = readFull(r, name); err != nil {
			return nil, 0, false
		}
		entries = append(entries, Entry{ID: id, Parent: parent, Attr: attr, Name: string(name)})
	}
	return entries, cachedNext, true
}
func readFull(r *bufio.Reader, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, e := r.Read(b[n:])
		n += m
		if e != nil {
			return n, e
		}
		if m == 0 {
			return n, fmt.Errorf("zero read")
		}
	}
	return n, nil
}
func writeCache(path string, idx *VolumeIndex) error {
	if idx.JournalID == 0 {
		return fmt.Errorf("journal unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 2*1024*1024)
	hdr := make([]byte, 32)
	copy(hdr, "SSP2")
	binary.LittleEndian.PutUint32(hdr[4:], idx.Serial)
	binary.LittleEndian.PutUint64(hdr[8:], idx.JournalID)
	binary.LittleEndian.PutUint64(hdr[16:], uint64(idx.NextUSN))
	binary.LittleEndian.PutUint64(hdr[24:], uint64(len(idx.Entries)))
	if _, err = w.Write(hdr); err != nil {
		f.Close()
		return err
	}
	for _, e := range idx.Entries {
		var rec [24]byte
		binary.LittleEndian.PutUint64(rec[0:], e.ID)
		binary.LittleEndian.PutUint64(rec[8:], e.Parent)
		binary.LittleEndian.PutUint32(rec[16:], e.Attr)
		binary.LittleEndian.PutUint32(rec[20:], uint32(len(e.Name)))
		if _, err = w.Write(rec[:]); err != nil {
			f.Close()
			return err
		}
		if _, err = w.WriteString(e.Name); err != nil {
			f.Close()
			return err
		}
	}
	if err = w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *App) copyTextDummy() {}
func ensureAdmin() bool {
	r, _, _ := procIsUserAnAdmin.Call()
	if r != 0 {
		return true
	}
	buf := make([]uint16, 32768)
	n, _, _ := procGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || int(n) >= len(buf) {
		return false
	}
	file := syscall.UTF16ToString(buf[:n])
	verb, arg := wptr("runas"), wptr("")
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(wptr(file))), uintptr(unsafe.Pointer(arg)), 0, SW_SHOWNORMAL)
	return false
}
func main() {
	// Win32 window creation and the message loop must stay on the same OS thread.
	// Without this, the Go scheduler can migrate the main goroutine after a syscall,
	// leaving the window created on one thread while GetMessage runs on another.
	runtime.LockOSThread()
	if !ensureAdmin() {
		return
	}
	// Per-monitor DPI awareness avoids blurry text and layout scaling on Windows 10/11.
	procSetProcessDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	icc := INITCOMMONCONTROLSEX{Size: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), ICC: 0x00000001 | 0x00000004 | 0x00000008}
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	inst, _, _ := procGetModuleHandleW.Call(0)
	// Soft neutral canvas, white cards, and a restrained navy header/blue action color.
	app.bgBrush, _, _ = procCreateSolidBrush.Call(rgb(246, 248, 252))
	app.cardBrush, _, _ = procCreateSolidBrush.Call(rgb(255, 255, 255))
	app.headerBrush, _, _ = procCreateSolidBrush.Call(rgb(24, 37, 59))
	cls := wptr("SwiftSearchPlusNativeWindow")
	cursor, _, _ := procLoadCursorW.Call(0, IDC_ARROW)
	icon, _, _ := procLoadIconW.Call(inst, 1) // Project-owned application icon is embedded as resource ID 1.
	wc := WNDCLASSEX{Size: uint32(unsafe.Sizeof(WNDCLASSEX{})), Style: 0, WndProc: syscall.NewCallback(wndProc), Instance: inst, Icon: icon, Cursor: cursor, Background: app.bgBrush, ClassName: cls, IconSm: icon}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	// App manifest requests common-controls v6 through default system theme; no external runtime files.
	title := wptr("SwiftSearch Plus")
	// Start wide enough for the metadata columns; clamp to the current display.
	getMetric := user32.NewProc("GetSystemMetrics")
	screenW, _, _ := getMetric.Call(0) // SM_CXSCREEN
	screenH, _, _ := getMetric.Call(1) // SM_CYSCREEN
	sysDPI := uintptr(96)
	if getDPI := user32.NewProc("GetDpiForSystem"); getDPI != nil {
		if d, _, _ := getDPI.Call(); d != 0 {
			sysDPI = d
		}
	}
	winW := uintptr(1420) * sysDPI / 96
	winH := uintptr(850) * sysDPI / 96
	if screenW > 200 && winW > screenW-48 {
		winW = screenW - 48
	}
	if screenH > 200 && winH > screenH-80 {
		winH = screenH - 80
	}
	if winW < 1000 {
		winW = 1000
	}
	if winH < 650 {
		winH = 650
	}
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, winW, winH, 0, 0, inst, 0)
	if hwnd == 0 {
		os.Exit(1)
	}
	procShowWindow.Call(hwnd, SW_SHOWNORMAL)
	procUpdateWindow.Call(hwnd)
	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
