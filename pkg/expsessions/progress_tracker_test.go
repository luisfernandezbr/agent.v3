package expsessions

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	jsonp "github.com/pinpt/go-common/json"
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

func assertProgressLines(t *testing.T, pt *ProgressTracker, want []ProgressLine) {
	t.Helper()
	got := pt.ProgressLines("/")
	want = want
	if !reflect.DeepEqual(want, got) {
		t.Errorf("wanted progress \n%v\ngot\n%v", want, got)
	}
}

func TestProgressTrackerProgressLines(t *testing.T) {

	pt := NewProgressTracker()
	pt.Update(testPath("org"), 1, 2)
	pt.Update(testPath("org"), 2, 2)

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, false, true},
	})

	pt.Update(testPath("org/1/repos"), 1, 3)

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, false, true},
		{"org/repos", 0, 2, false, true},
		{"org/1/repos/meta", 1, 3, false, true},
	})

	pt.Update(testPath("org/1/repos"), 3, 3)

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, false, true},
		{"org/repos", 0, 2, false, true},
		{"org/1/repos/meta", 3, 3, false, true},
	})

	pt.Done(testPath("org/1/repos"))

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, false, true},
		{"org/repos", 1, 2, false, true},
		{"org/1/repos/meta", 3, 3, true, true},
	})

	pt.Done(testPath("org/2/repos"))

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, false, true},
		{"org/repos", 2, 2, true, true},
		{"org/1/repos/meta", 3, 3, true, true},
		{"org/2/repos/meta", 0, 0, true, true},
	})

	pt.Done(testPath("org"))

	assertProgressLines(t, pt, []ProgressLine{
		{"org/meta", 2, 2, true, true},
		{"org/repos", 2, 2, true, true},
		{"org/1/repos/meta", 3, 3, true, true},
		{"org/2/repos/meta", 0, 0, true, true},
	})
}

func TestProgressLinesToNested(t *testing.T) {
	lines := []ProgressLine{
		{"org/meta", 2, 2, true, true},
		{"org/repos", 2, 2, true, true},
		{"org/1/repos/meta", 3, 3, true, true},
		{"org/2/repos/meta", 0, 0, true, true},
	}
	got := progressLinesToNested(lines, "/")
	want := `
	{
		"nested": {
			"org": {
				"nested": {
					"meta": {"c":2,"t":2,"done":true,"summary":true},
					"repos": {"c":2,"t":2,"done":true,"summary":true},
					"1": {
						"nested": {
							"repos": {
								"nested": {
									"meta": {"c":3,"t":3,"done":true,"summary":true}
								}
							}
						}
					},
					"2": {
						"nested": {
							"repos": {
								"nested": {
									"meta": {"done":true,"summary":true}
								}
							}
						}
					}
				}
			}
		}
	}	
	`
	var wantObj *ProgressStatus
	err := json.Unmarshal([]byte(want), &wantObj)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON := jsonp.Stringify(got, true)
	wantJSON := jsonp.Stringify(wantObj, true)
	if gotJSON != wantJSON {
		t.Errorf("want != got, got data\n%v\nwant\n%v", gotJSON, wantJSON)
	}
}

func testPath(str string) []string {
	return strings.Split(str, "/")
}
