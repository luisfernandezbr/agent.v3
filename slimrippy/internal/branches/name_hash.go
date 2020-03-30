package branches

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/pinpt/agent/slimrippy/internal/branchmeta"
)

type nameAndHash struct {
	Name   string
	Commit string
}

type namesAndHashes []nameAndHash

func (s namesAndHashes) Chan() chan nameAndHash {
	res := make(chan nameAndHash)
	go func() {
		for _, v := range s {
			res <- v
		}
		close(res)
	}()
	return res
}

func (s *Process) getNamesAndHashes() (res namesAndHashes, _ error) {
	ctx := context.Background()
	res0, err := branchmeta.GetAll(ctx, s.opts.RepoDir, false)
	if err != nil {
		return res, err
	}
	for _, item := range res0 {
		res = append(res, nameAndHash{Name: item.Name, Commit: item.Commit})
	}
	return res, nil
}

func execCommand(command string, dir string, args []string) ([]byte, error) {
	out := bytes.NewBuffer(nil)
	c := exec.Command(command, args...)
	c.Dir = dir
	c.Stdout = out
	err := c.Run()
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
