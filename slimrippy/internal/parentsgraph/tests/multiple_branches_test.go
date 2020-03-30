package tests

import (
	"testing"
)

func TestMultipleBranches1(t *testing.T) {
	tc := NewTest(t, "multiple_branches", nil)
	pg := tc.Run()

	c1 := "bdf8c8cfa9c027e58f1aea5c532ba0e9ef74bc4c"
	c2 := "d3a93f475772c90918ebc34e144e1c3554163a9f"
	c3 := "7c6eba56ba8616ee903f2394553c022d6d3046bf"
	c4 := "3f18a2ea07832a18d0645df2aa666b339cee1a06"

	wantParents := map[string][]string{
		c1: nil,
		c2: []string{c1},
		c3: []string{c1},
		c4: []string{c1},
	}

	wantChildren := map[string][]string{
		c1: []string{c2, c3, c4},
		c2: nil,
		c3: nil,
		c4: nil,
	}

	assertResult(t, pg, wantParents, wantChildren)
}
