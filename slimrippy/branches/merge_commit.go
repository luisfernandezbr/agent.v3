package branches

import (
	"fmt"

	"github.com/pinpt/agent/slimrippy/parentsgraph"
)

func getMergeCommit(gr *parentsgraph.Graph, reachableFromHead reachableFromHead, branchHead string) (_ string, rerr error) {
	children, ok := gr.Children[branchHead]
	if !ok {
		rerr = fmt.Errorf("commit not found in tree: %v", branchHead)
		return
	}
	for _, ch := range children {
		if reachableFromHead[ch] {
			return ch, nil
		}
	}
	return "", nil
}
