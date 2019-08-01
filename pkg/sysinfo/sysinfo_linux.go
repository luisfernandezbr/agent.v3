// +build linux

package sysinfo

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
)

var dequoteRegexp = regexp.MustCompile(`"(.*)"`)

func dequote(s string) string {
	if dequoteRegexp.MatchString(s) {
		return dequoteRegexp.FindStringSubmatch(s)[1]
	}
	return s
}

// GetSystemInfo returns the SystemInfo details
func GetSystemInfo() SystemInfo {
	var buf bytes.Buffer
	c := exec.Command("cat /etc/*-release")
	c.Stdout = &buf
	c.Run()
	kv := make(map[string]string)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line != "" {
			tok := strings.Split(line, "=")
			if len(tok) > 1 {
				key, value := strings.TrimSpace(tok[0]), strings.TrimSpace(tok[1])
				kv[key] = value
			}
		}
	}
	def := getDefault()
	def.Name = kv["NAME"]
	def.Version = kv["VERSION_ID"]
	return def
}

/*
PRETTY_NAME="Debian GNU/Linux 9 (stretch)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (stretch)"
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
*/
