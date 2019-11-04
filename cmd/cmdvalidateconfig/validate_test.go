package cmdvalidateconfig

import "testing"

func TestURLWithoutCreds(t *testing.T) {
	cases := []struct {
		In   string
		Want string
	}{
		{
			In:   "https://example.com",
			Want: "https://example.com",
		},
		{
			In:   "https://user:pass@example.com",
			Want: "https://example.com",
		},
	}
	for _, c := range cases {
		got, err := urlWithoutCreds(c.In)
		if err != nil {
			t.Error(err)
		}
		if got != c.Want {
			t.Errorf("wanted %v, got %v", c.Want, got)
		}
	}
}
