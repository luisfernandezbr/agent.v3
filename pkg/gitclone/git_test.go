package gitclone

import (
	"testing"
)

func TestDirNameFromURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`https://github.com/user1/repo2.git`, `github-com-user1-repo2-d4ded286334d210776ead49ef71f8dff2f6b6452`},
	}
	for _, v := range cases {
		got, err := dirNameFromURL(v.in)
		if err != nil {
			t.Error(err)
		}
		if got != v.want {
			t.Errorf("wanted %v, got %v, for case %v", v.want, got, v.in)
		}
	}
}

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
