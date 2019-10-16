package expsessions

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ProgressTracker struct {
	tree Children
	mu   sync.Mutex
}

// map[node_id]map[subtype]*Node
type Children map[string]map[string]*Node

type Node struct {
	Children Children
	Current  int
	Total    int
	Done     bool
}

func NewProgressTracker() *ProgressTracker {
	s := &ProgressTracker{}
	s.tree = map[string]map[string]*Node{}
	return s
}

func (s *ProgressTracker) Update(path []string, current, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(path) == 0 {
		return
	}
	node := s.findNode(path)
	node.Current = current
	node.Total = total
}

func (s *ProgressTracker) findNode(path []string) *Node {
	c := path
	head := func() (id string, kind string, rest []string) {
		if len(c)%2 == 1 {
			return "", c[0], c[1:]
		}
		return c[0], c[1], c[2:]
	}

	tree := s.tree
	var n *Node
	for {
		if len(c) == 0 {
			break
		}
		var id, kind string
		id, kind, c = head()
		//fmt.Println("id, kind", id, kind, c, "orig", path)
		var ok bool
		if _, ok := tree[id]; !ok {
			tree[id] = map[string]*Node{}
		}
		n, ok = tree[id][kind]
		if !ok {
			n = &Node{}
			n.Children = map[string]map[string]*Node{}
			tree[id][kind] = n
		}
		tree = n.Children
	}
	return n
}

func (s *ProgressTracker) Done(path []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.findNode(path)
	node.Done = true
}

type kv struct {
	K string
	V int
}

func sortMap(m map[string]int) (res []kv) {
	for k, v := range m {
		res = append(res, kv{k, v})
	}
	sort.Slice(res, func(i, j int) bool {
		a := res[i]
		b := res[j]
		return a.K < b.K
	})
	return res
}

func formatCurrTotal(curr, total int) string {
	if total == 0 {
		return strconv.Itoa(curr) + "/?"
	}
	return strconv.Itoa(curr) + "/" + strconv.Itoa(total)
}

func (s *ProgressTracker) InProgressString() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res []string
	l := func(str string) {
		res = append(res, str)
		res = append(res, "\n")
	}

	type sortv struct {
		ID      string
		Subtype string
		Node    *Node
	}

	iterSorted := func(tree Children) (res []sortv) {

		var uids []string
		for id := range tree {
			uids = append(uids, id)
		}
		sort.Strings(uids)
		for _, id := range uids {
			m2 := tree[id]
			var usubs []string
			for sub := range m2 {
				usubs = append(usubs, sub)
			}
			sort.Strings(usubs)
			for _, sub := range usubs {
				node := m2[sub]
				res = append(res, sortv{
					ID:      id,
					Subtype: sub,
					Node:    node,
				})
			}
		}
		return
	}

	var rec func(node *Node, prefix string)

	rec = func(node *Node, prefix string) {
		if node.Done {
			return
		}

		l(prefix + " " + formatCurrTotal(node.Current, node.Total) + " meta")

		subs := map[string]int{}
		for _, m2 := range node.Children {
			for subtype, n := range m2 {
				if subtype == "" {
					panic("invalid")
				}
				if _, ok := subs[subtype]; !ok {
					subs[subtype] = 0
				}
				if n.Done {
					subs[subtype]++
				}
			}
		}

		for _, kv := range sortMap(subs) {
			total := node.Total
			curr := kv.V
			if curr == total {
				continue
			}
			l(prefix + " " + formatCurrTotal(curr, total) + " " + kv.K)
		}

		for _, v := range iterSorted(node.Children) {
			id := v.ID
			subtype := v.Subtype
			node := v.Node
			if subtype == "" {
				panic("invalid")
			}
			rec(node, prefix+"/"+id+"/"+subtype)
		}
	}

	for _, v := range iterSorted(s.tree) {
		rec(v.Node, v.Subtype)
	}

	return strings.Join(res, "")
}

type ProgressLine struct {
	Path      string `json:"path"`
	Current   int    `json:"current"`
	Total     int    `json:"total"`
	Done      bool   `json:"done"`
	IsSummary bool   `json:"is_summary"`
}

func (s *ProgressTracker) ProgressLines(pathSep string, skipDone bool) (res []ProgressLine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type sortv struct {
		ID      string
		Subtype string
		Node    *Node
	}

	iterSorted := func(tree Children) (res []sortv) {

		var uids []string
		for id := range tree {
			uids = append(uids, id)
		}
		sort.Strings(uids)
		for _, id := range uids {
			m2 := tree[id]
			var usubs []string
			for sub := range m2 {
				usubs = append(usubs, sub)
			}
			sort.Strings(usubs)
			for _, sub := range usubs {
				node := m2[sub]
				res = append(res, sortv{
					ID:      id,
					Subtype: sub,
					Node:    node,
				})
			}
		}
		return
	}

	l2 := func(path string, current int, total int, done bool, isSummary bool) {
		res = append(res, ProgressLine{path, current, total, done, isSummary})
	}

	var rec func(node *Node, prefix string)

	rec = func(node *Node, prefix string) {
		if skipDone {
			if node.Done {
				return
			}
		}

		l2(prefix+pathSep+"meta", node.Current, node.Total, node.Done, true)

		subs := map[string]int{}
		for _, m2 := range node.Children {
			for subtype, n := range m2 {
				if subtype == "" {
					panic("invalid")
				}
				if _, ok := subs[subtype]; !ok {
					subs[subtype] = 0
				}
				if n.Done {
					subs[subtype]++
				}
			}
		}

		for _, kv := range sortMap(subs) {
			total := node.Total
			curr := kv.V
			done := curr == total
			if skipDone {
				if done {
					continue
				}
			}
			l2(prefix+pathSep+kv.K, curr, total, done, true)
		}

		for _, v := range iterSorted(node.Children) {
			id := v.ID
			subtype := v.Subtype
			node := v.Node
			if subtype == "" {
				panic("invalid")
			}
			rec(node, prefix+pathSep+id+pathSep+subtype)
		}
	}

	for _, v := range iterSorted(s.tree) {
		rec(v.Node, v.Subtype)
	}

	return
}

type ProgressStatus struct {
	Current   int                        `json:"c,omitempty"`
	Total     int                        `json:"t,omitempty"`
	Done      bool                       `json:"done,omitempty"`
	IsSummary bool                       `json:"summary,omitempty"`
	Nested    map[string]*ProgressStatus `json:"nested,omitempty"`
}

func progressLinesToNested(lines []ProgressLine, sep string) *ProgressStatus {
	res := &ProgressStatus{}
	res.Nested = map[string]*ProgressStatus{}
	for _, l := range lines {
		p := strings.Split(l.Path, sep)
		c := res
		for {
			if len(p) == 0 {
				break
			}
			h := p[0]
			p = p[1:]
			var ok bool
			var node *ProgressStatus
			node, ok = c.Nested[h]
			if !ok {
				nn := &ProgressStatus{}
				nn.Nested = map[string]*ProgressStatus{}
				c.Nested[h] = nn
				node = nn
			}
			c = node
		}
		c.Current = l.Current
		c.Total = l.Total
		c.Done = l.Done
		c.IsSummary = l.IsSummary
	}
	return res
}

func progressLinesToNestedMap(lines []ProgressLine, sep string) map[string]interface{} {
	data := progressLinesToNested(lines, sep)
	var rec func(data *ProgressStatus, cont bool) map[string]interface{}
	rec = func(data *ProgressStatus, cont bool) map[string]interface{} {
		res := map[string]interface{}{}
		totals := map[string]interface{}{}
		for k, v := range data.Nested {
			if k == "meta" {
				res[k] = map[string]interface{}{
					"c":    v.Current,
					"t":    v.Total,
					"done": v.Done,
				}
				continue
			}
			if !cont && !strings.Contains(k, ":") {
				totals[k] = map[string]interface{}{
					"c":    v.Current,
					"t":    v.Total,
					"done": v.Done,
				}
				continue
			}
			res[k] = rec(v, !cont)
		}
		if len(totals) != 0 {
			res["totals"] = totals
		}
		return res
	}
	return rec(data, true)
}

func (s *ProgressTracker) ProgressLinesNested(skipDone bool) *ProgressStatus {
	sep := "@@@"
	lines := s.ProgressLines(sep, skipDone)
	return progressLinesToNested(lines, sep)
}

func (s *ProgressTracker) ProgressLinesNestedMap(skipDone bool) map[string]interface{} {
	sep := "@@@"
	lines := s.ProgressLines(sep, skipDone)
	return progressLinesToNestedMap(lines, sep)
}
