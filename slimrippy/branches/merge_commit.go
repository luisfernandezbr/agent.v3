package branches

import "github.com/pinpt/agent/slimrippy/parentsgraph"

func getMergeCommit(gr *parentsgraph.Graph, reachableFromHead reachableFromHead, branchHead string) string {
	children, ok := gr.Children[branchHead]
	if !ok {
		panic("commit not found in tree")
	}
	for _, ch := range children {
		if reachableFromHead[ch] {
			return ch
		}
	}
	return ""
}
