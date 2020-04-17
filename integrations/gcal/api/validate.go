package api

func (s *api) Validate() error {
	var res interface{}
	err := s.get("/colors", queryParams{}, &res)
	return err
}
