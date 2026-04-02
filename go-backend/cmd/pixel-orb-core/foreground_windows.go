package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

type ForegroundWindow struct {
	Title       string `json:"title"`
	ProcessName string `json:"processName"`
	ProcessID   int    `json:"processId"`
}

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessID   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
)

const processQueryLimitedInformation = 0x1000

func queryForegroundWindow() (ForegroundWindow, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ForegroundWindow{}, nil
	}

	titleLength, _, _ := procGetWindowTextLengthW.Call(hwnd)
	titleBuffer := make([]uint16, titleLength+1)
	if len(titleBuffer) > 0 {
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuffer[0])), uintptr(len(titleBuffer)))
	}

	var processID uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&processID)))

	return ForegroundWindow{
		Title:       strings.TrimSpace(syscall.UTF16ToString(titleBuffer)),
		ProcessName: strings.TrimSpace(readProcessName(processID)),
		ProcessID:   int(processID),
	}, nil
}

func readProcessName(processID uint32) string {
	if processID == 0 {
		return ""
	}

	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(processID))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buffer := make([]uint16, syscall.MAX_PATH)
	bufferSize := uint32(len(buffer))
	result, _, _ := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&bufferSize)),
	)
	if result == 0 || bufferSize == 0 {
		return ""
	}

	fullPath := syscall.UTF16ToString(buffer[:bufferSize])
	baseName := filepath.Base(fullPath)
	return strings.TrimSuffix(baseName, filepath.Ext(baseName))
}
