package e2etests

import (
	"testing"
	"time"

	"github.com/pinpt/agent/slimrippy/internal/branchmeta"
)

func TestBranchesBasic1(t *testing.T) {
	test := NewTest(t, "basic1", false)
	got := test.Run()

	want := []branchmeta.Branch{
		{
			Name:   "a",
			Commit: "9b39087654af70197f68d0b3d196a4a20d987cd6",
		},
	}

	assertResult(t, want, got)
}

func TestBranchesIncludeDefault(t *testing.T) {
	test := NewTest(t, "basic1", true)
	got := test.Run()

	want := []branchmeta.Branch{
		{
			Name:   "a",
			Commit: "9b39087654af70197f68d0b3d196a4a20d987cd6",
		},
		{
			Name:   "master",
			Commit: "33e223d1fd8393dc98596727d370e51e7b3b7fba",
		},
	}
	assertResult(t, want, got)
}

func parseTime(s string) time.Time {
	res, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return res
}
