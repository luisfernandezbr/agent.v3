// +build windows

package sysinfo

import (
	"fmt"
	"log"
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
