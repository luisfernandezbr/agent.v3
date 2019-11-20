package cmdvalidate

import "testing"

func TestGitVersion(t *testing.T) {
	cases := []struct {
		Version string
		Min     string
		Want    bool
		Err     error
	}{
		{
			Version: "git version 2.13.0\n", // newline
			Min:     "2.13.0",
			Want:    true,
			Err:     nil,
		},
		{
			Version: "git version 2.13.0 (Apple Git-117)", // macos
			Min:     "2.13.0",
			Want:    true,
			Err:     nil,
		},
		{
			Version: "git version 2.12.1 (Apple Git-117)",
			Min:     "2.13.0",
			Want:    false,
			Err:     nil,
		},
		{
			Version: "git version 2.23.0.windows.1", // windows
			Min:     "2.13.0",
			Want:    true,
			Err:     nil,
		},
		{
			Version: "git version 2.22.0", // linux
			Min:     "2.13.0",
			Want:    true,
			Err:     nil,
		},
	}

	for _, c := range cases {
		got, err := gitVersionGteq(c.Version, c.Min)
		if err != c.Err {
			t.Errorf("wanted err = %v, got %v", c.Err, err)
		}
		if c.Want != got {
			t.Errorf("wanted = %v, got %v", c.Want, got)
		}
	}
}
