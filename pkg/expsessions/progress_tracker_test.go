package expsessions

import (
	"strings"
	"testing"
)

func assertProgress(t *testing.T, pt *ProgressTracker, want string) {
	t.Helper()
	got := stripLineSpaces(pt.InProgressString())
	want = stripLineSpaces(want)
	if got != want {
		t.Errorf("wanted progress \n%v\ngot\n%v", want, got)
	}
}

func stripLineSpaces(str string) string {
	lines := strings.Split(str, "\n")
	var res []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) == 0 {
			continue
		}
		res = append(res, l)
	}
	return strings.Join(res, "\n")
}

func TestProgressTrackerBasic(t *testing.T) {
	pt := NewProgressTracker()
	pt.Update(testPath("org"), 1, 2)
	pt.Update(testPath("org"), 2, 2)

	assertProgress(t, pt, `
		org 2/2 meta
	`)

	pt.Update(testPath("org/1/repos"), 1, 3)

	assertProgress(t, pt, `
		org 2/2 meta
		org 0/2 repos
		org/1/repos 1/3 meta
	`)

	pt.Update(testPath("org/1/repos"), 3, 3)

	pt.Done(testPath("org/1/repos"))

	assertProgress(t, pt, `
		org 2/2 meta
		org 1/2 repos
	`)

	pt.Done(testPath("org/2/repos"))

	assertProgress(t, pt, `
		org 2/2 meta
	`)

	pt.Done(testPath("org"))

	assertProgress(t, pt, ``)
}

func testPath(str string) []string {
	return strings.Split(str, "/")
}
