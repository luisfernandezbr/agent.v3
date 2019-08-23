package api

import "testing"

func TestGetNextFromLinkHeader(t *testing.T) {

	data := `<https://hostname.com/api/v3/user/repos?page=3&per_page=100>; rel="next",
	<https://hostname.com/api/v3/user/repos?page=50&per_page=100>; rel="last"
  `

	res, err := getNextFromLinkHeader(data)
	if err != nil {
		t.Error(err)
	}
	if res != "https://hostname.com/api/v3/user/repos?page=3&per_page=100" {
		t.Errorf("invalid result %v", res)
	}
}
