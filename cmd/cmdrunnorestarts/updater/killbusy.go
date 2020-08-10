package updater

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func killBusyProcesses(err error) error {

	var re = regexp.MustCompile(`(integrations\/.+)\/`)

	var file string
	for _, match := range re.FindAllStringSubmatch(err.Error(), -1) {
		if len(match) >= 2 {
			file = match[1]
		}
	}

	if file == "" {
		return fmt.Errorf("no file found, err %s", err)
	}

	strCommand := fmt.Sprintf("lsof +D %s", file)
	c := exec.Command("sh", "-c", strCommand)
	c.Stderr = os.Stdout
	bts, err := c.Output()
	if err != nil && err.Error() != "exit status 1" {
		return fmt.Errorf("error running ps, file %s, error %s", file, err)
	}

	killPID := "kill -15 %s"
	sudoKillPID := "sudo kill -15 %s"

	for _, str := range strings.Split(string(bts), "\n") {
		var re = regexp.MustCompile(`\d+`)

		var res = re.FindAllString(str, -1)

		if len(res) > 0 {
			match := res[0]
			c := exec.Command("sh", "-c", fmt.Sprintf(killPID, match))
			c.Stderr = os.Stdout
			_, err0 := c.Output()
			if err0 != nil {
				c := exec.Command("sh", "-c", fmt.Sprintf(sudoKillPID, match))
				c.Stderr = os.Stdout
				_, err = c.Output()
				if err != nil {
					return fmt.Errorf("error running kill, errors %s, %s", err0, err)
				}
			}
		}
	}

	return nil
}
