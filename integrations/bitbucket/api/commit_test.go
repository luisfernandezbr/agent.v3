package api

import (
	"testing"
)

func TestParseSprintTime(t *testing.T) {
	cases := []struct {
		In        string
		NameWant  string
		EmailWant string
	}{
		{
			"^_623s#%1na3%^ <01em2a3il4%^&%>",
			"^_623s#%1na3%^",
			"01em2a3il4%^&%",
		},
		{
			"^_623s#%1na3%^ <01em2a3il4%^&%>",
			"^_623s#%1na3%^",
			"01em2a3il4%^&%",
		},
		{
			"Jose C Ordaz <cordaz@pinpoint.com>",
			"Jose C Ordaz",
			"cordaz@pinpoint.com",
		},
	}
	for _, c := range cases {
		name, email := getNameAndEmail(c.In)
		if name != c.NameWant {
			t.Errorf("wanted [%v], got [%v]", c.NameWant, name)
		}
		if email != c.EmailWant {
			t.Errorf("wanted [%v], got [%v]", c.EmailWant, email)
		}
	}
}
