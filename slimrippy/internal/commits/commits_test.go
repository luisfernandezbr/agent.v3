package commits

import (
	"context"
	"testing"

	. "github.com/pinpt/agent/slimrippy/testutil"
)

func TestBasic(t *testing.T) {
	var got []Commit
	var err error
	NewTest(t, "basic").Run(func(opts Opts) {
		got, _, err = CommitsSlice(context.Background(), opts)
	})
	if err != nil {
		t.Fatal(err)
	}

	c1 := Commit{}
	c1.SHA = "b4dadc54e312e976694161c2ac59ab76feb0c40d"
	c1.Message = "c1"
	c1.Authored.Name = "User1"
	c1.Authored.Email = "user1@example.com"
	c1.Authored.Date = ParseGitDate("Tue Nov 27 21:55:36 2018 +0100")
	c1.Committed = c1.Authored

	c2 := Commit{}
	c2.SHA = "69ba50fff990c169f80de96674919033a0a9b66d"
	c2.Message = "c2"
	c2.Authored.Name = "User2"
	c2.Authored.Email = "user2@example.com"
	c2.Authored.Date = ParseGitDate("Tue Nov 27 21:56:11 2018 +0100")
	c2.Committed = c2.Authored

	assertResult(t, []Commit{c2, c1}, got)
}
