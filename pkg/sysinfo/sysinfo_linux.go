// +build linux

package sysinfo

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pinpt/go-common/v10/fileutil"
	"github.com/pinpt/go-common/v10/hash"
)

var dequoteRegexp = regexp.MustCompile(`"(.*)"`)

func dequote(s string) string {
	if dequoteRegexp.MatchString(s) {
		return dequoteRegexp.FindStringSubmatch(s)[1]
	}
	return s
}

var fileLocations = []string{
	"/etc/machine-id",
	"/sys/class/dmi/id/board_serial",
	"/sys/class/dmi/id/chassis_serial",
	"/proc/sys/kernel/random/boot_id",
}

func readFileIfExists(fn string) string {
	if fileutil.FileExists(fn) {
		buf, _ := ioutil.ReadFile(fn)
		return string(buf)
	}
	return ""
}

// GetSystemInfo returns the SystemInfo details
func GetSystemInfo(root string) SystemInfo {
	files, _ := fileutil.FindFiles("/etc", regexp.MustCompile(`-release$`))
	kv := make(map[string]string)
	for _, fn := range files {
		b, _ := ioutil.ReadFile(fn)
		for _, line := range strings.Split(string(b), "\n") {
			if line != "" {
				tok := strings.Split(line, "=")
				if len(tok) > 1 {
					key, value := strings.TrimSpace(tok[0]), strings.TrimSpace(tok[1])
					kv[key] = dequote(value)
				}
			}
		}
	}
	def := getDefault(root)
	def.Name = kv["NAME"]
	def.Version = kv["VERSION_ID"]
	insidedocker := fileutil.FileExists("/proc/self/cgroup")
	if def.ID == "" {
		if insidedocker {
			// is is docker, pull out the unique id for it
			b, _ := ioutil.ReadFile("/proc/self/cgroup")
			line := strings.Split(string(b), "\n")[0] // get the 64-char long id that docker creates for each container
			tok := strings.Split(line, "/")
			def.ID = tok[len(tok)-1]
		} else {
			for _, fn := range fileLocations {
				str := readFileIfExists(fn)
				if str != "" && str != "None" {
					def.ID = str
					break
				}
			}
		}
		if def.ID == "" {
			// try and just hash all the mac addresses + the uname -a to create a consistent
			// and hopefully random unique id
			var buf strings.Builder
			c := exec.Command("uname", "-a")
			c.Stdout = &buf
			c.Run()
			args := []interface{}{buf.String()}
			buf.Reset()
			c = exec.Command("hostname")
			c.Stdout = &buf
			c.Run()
			args = append(args, buf.String())
			devices, _ := fileutil.FindFiles("/sys/class/net/", regexp.MustCompile(".*"))
			for _, dir := range devices {
				fn := filepath.Join(dir, "address")
				args = append(args, readFileIfExists(fn))
			}
			args = append(args, readFileIfExists("/proc/version"))
			// hash and then iterate to create 64 char long
			def.ID = hash.Values(args...)
			def.ID += hash.Values(def.ID)
			def.ID += hash.Values(def.ID)
			def.ID += hash.Values(def.ID)
		}
	}
	if def.ID == "" {
		panic("Couldn't generate a unique ID for this machine. Please set the environment variable PP_UUID to a 64-character long unique identifier")
	}
	if insidedocker {
		def.Name += " (Docker)"
	}
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
