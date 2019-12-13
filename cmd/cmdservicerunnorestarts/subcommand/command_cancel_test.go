package subcommand

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestCommandCancel(t *testing.T) {
	logger := hclog.NewNullLogger()

	started := time.Now()
	killtime := 3 * time.Second

	// create a temp dir for testing only
	gitclone, err := ioutil.TempDir("", "test_cancel")
	assert.NoError(t, err)
	// cleanup
	defer func() {
		assert.NoError(t, os.RemoveAll(gitclone))
	}()

	err = os.MkdirAll(gitclone, 0755)
	assert.NoError(t, err)

	// pick a command that will take some time to run
	cmd := exec.Command("git", "clone", "git@github.com:pinpt/agent.git", gitclone)
	assert.NoError(t, cmd.Start())

	// insert command as soon as it's started
	addProcess(logger, "git clone", cmd.Process)
	go func() {
		// kill the command while it's processing
		time.Sleep(killtime)
		removeProcess(logger, "git clone")
	}()

	// we should get the "killed" signal if this was manually killed
	assert.EqualError(t, cmd.Wait(), "signal: killed")

	// round to seconds
	ellapsed := time.Since(started).Seconds()
	target := killtime.Seconds()
	assert.Equal(t, int(ellapsed), int(target))

}
