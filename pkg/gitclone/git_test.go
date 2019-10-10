package gitclone

import (
	"testing"
)

func TestEscapeForFS(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{``, ``},
		{`github.com/user/repo1`, "github-com-user-repo1"},
		{`\x/x`, `-x-x`},
		{`:a@`, `-a-`},
	}
	for _, v := range cases {
		got := escapeForFS(v.in)
		if got != v.want {
			t.Errorf("wanted %v, got %v, for case %v", v.want, got, v.in)
		}
	}
}
