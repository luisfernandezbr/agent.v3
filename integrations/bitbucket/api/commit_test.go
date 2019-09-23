package api

import (
	"testing"
)

func TestGetNameAndEmail(t *testing.T) {
	cases := []struct {
		In        string
		NameWant  string
		EmailWant string
	}{
		{
			"name <name@email.com>",
			"name",
			"name@email.com",
		},
		{
			"<name@email.com>",
			"",
			"name@email.com",
		},
		{
			"name <>",
			"name",
			"",
		},
		{
			"",
			"",
			"",
		},
		{
			"<",
			"",
			"",
		},
	}
	for i, c := range cases {
		name, email := getNameAndEmail(c.In)
		if name != c.NameWant {
			t.Errorf("wanted [%v], got [%v]", c.NameWant, name)
		}
		if email != c.EmailWant {
			t.Errorf("%d wanted [%v], got [%v]", i, c.EmailWant, email)
		}
	}
}
