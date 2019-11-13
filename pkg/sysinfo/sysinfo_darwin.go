// +build darwin

package sysinfo

import (
	"bytes"
	"os/exec"
	"strings"
)

// GetSystemInfo returns the SystemInfo details
func GetSystemInfo(root string) SystemInfo {
	var buf bytes.Buffer
	c := exec.Command("sw_vers")
	c.Stdout = &buf
	c.Run()
	kv := make(map[string]string)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line != "" {
			tok := strings.Split(line, ":")
			if len(tok) > 1 {
				key, value := strings.TrimSpace(tok[0]), strings.TrimSpace(tok[1])
				kv[key] = value
			}
		}
	}
	def := getDefault(root)
	def.Name = kv["ProductName"]
	def.Version = kv["ProductVersion"]
	return def
}

/*
ProductName:	Mac OS X
ProductVersion:	10.13.5
BuildVersion:	17F77
*/
