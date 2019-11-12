package gitclone

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// TestClone tried to clone repo based on the url passed. If it fails returns an error.
// Does not do the full clone, exists after it confirms that remote is sending data.
func TestClone(logger hclog.Logger, url string, tempDirRoot string) error {

	tempDir, err := ioutil.TempDir(tempDirRoot, "validate-repo-clone-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args := []string{
		"clone", "--progress", url, tempDir,
	}

	cloneArgs := cloneArgs(url)
	args = append(args, cloneArgs...)

	logger.Debug("additional clone args", "args", cloneArgs)

	c := exec.CommandContext(ctx, "git", args...)
	var outBuf bytes.Buffer
	c.Stdout = &outBuf
	stderr, _ := c.StderrPipe()

	err = c.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)
	var output []byte
	for scanner.Scan() {
		output = append(output, scanner.Bytes()...)

		line := scanner.Text()

		if strings.Contains(line, "Receiving objects:") ||
			strings.Contains(line, "Counting objects:") ||
			strings.Contains(line, "Enumerating objects:") ||
			strings.Contains(line, "You appear to have cloned an empty repository") {
			// If we see one of these lines, it means credentials are valid

			return nil
		}

	}

	if err := scanner.Err(); err != nil {
		return err
	}

	outputStr, err := RedactCredsInText(string(output), url)
	if err != nil {
		return err
	}

	return errors.New(outputStr)
}
