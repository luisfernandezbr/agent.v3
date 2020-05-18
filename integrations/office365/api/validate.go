package api

import "errors"

func (s *api) Validate() error {
	var res []struct {
		ID string `json:"id"`
	}
	err := s.get("/me", queryParams{}, &res)
	if err != nil {
		return err
	}
	if res[0].ID == "" {
		return errors.New("validate failed")
	}
	return nil
}
