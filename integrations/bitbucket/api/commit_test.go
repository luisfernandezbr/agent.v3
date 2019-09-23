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
func TestValidateIndex(t *testing.T) {
	cases := []struct {
		InStr   string
		InIndex int
		Want    bool
	}{
		{
			"str",
			0,
			true,
		},
		{
			"str",
			1,
			true,
		},
		{
			"str",
			3,
			true,
		},
		{
			"str",
			-1,
			false,
		},
		{
			"str",
			4,
			false,
		},
	}
	for i, c := range cases {
		validate := validateIndex(c.InStr, c.InIndex)
		if !validate && c.Want {
			t.Errorf("%d: StrIn[%s], Index[%d]  wanted [%v], got [%v]", i, c.InStr, c.InIndex, c.Want, validate)
		}
	}
}

func TestGetSubstring(t *testing.T) {
	cases := []struct {
		InStr     string
		InitIndex int
		EndIndex  int
		Want      string
	}{
		{
			"str",
			0,
			1,
			"s",
		},
		{
			"str",
			0,
			3,
			"str",
		},
		{
			"str",
			-1,
			3,
			"",
		},
		{
			"str",
			1,
			4,
			"",
		},
		{
			"str",
			3,
			2,
			"",
		},
	}
	for i, c := range cases {
		res := getSubstring(c.InStr, c.InitIndex, c.EndIndex)
		if res != c.Want {
			t.Errorf("%d: StrIn[%s], InitIndex[%d]  EndIndex[%d] Wanted [%v], got [%v]", i, c.InStr, c.InitIndex, c.EndIndex, c.Want, res)
		}
	}
}
