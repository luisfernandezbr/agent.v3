// +build windows

package sysinfo

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"syscall"
	"unsafe"
)

// GetSystemInfo returns the SystemInfo details
func GetSystemInfo() SystemInfo {
	dll := syscall.MustLoadDLL("kernel32.dll")
	p := dll.MustFindProc("GetVersion")
	v, _, _ := p.Call()
	def := getDefault()
	def.Name = "Windows"
	def.Version = fmt.Sprintf("%d.%d (Build %d)", byte(v), uint8(v>>8), uint16(v>>16))
	return def
}

func getFreeSpace(wd string) uint64 {
	kernel32, err := syscall.LoadLibrary("Kernel32.dll")
	if err != nil {
		log.Panic(err)
	}
	defer syscall.FreeLibrary(kernel32)
	GetDiskFreeSpaceEx, err := syscall.GetProcAddress(syscall.Handle(kernel32), "GetDiskFreeSpaceExW")

	if err != nil {
		log.Panic(err)
	}

	lpFreeBytesAvailable := int64(0)
	lpTotalNumberOfBytes := int64(0)
	lpTotalNumberOfFreeBytes := int64(0)

	syscall.Syscall6(uintptr(GetDiskFreeSpaceEx), 4,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(wd))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)), 0, 0)

	return uint64(lpFreeBytesAvailable)
}

// GetAvailablePath returns a valid path for the largest disk available
func GetAvailablePath() string {
	// Based on DriveType property, currently we support Removable and Local disks.
	// https://docs.microsoft.com/en-us/windows/win32/cimwin32prov/win32-logicaldisk
	cmd := exec.Command("WMIC", "LOGICALDISK", "where", "DriveType=2 or DriveType=3", "get", "DeviceID")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := bytes.Split(out, []byte{13, 13, 10})
	if len(lines) < 2 {
		return ""
	}

	var path string
	var higher, size uint64

	for _, line := range lines[1 : len(lines)-2] {
		line = bytes.Replace(line, []byte{32}, []byte{}, -1)
		line = append(line, []byte("\\")...)
		size = getFreeSpace(string(line))
		if size > higher {
			higher = size
			path = string(line)
		}
	}
	return path
}
