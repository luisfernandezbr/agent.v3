package api

import (
	"strings"

	"github.com/pinpt/integration-sdk/calendar"
)

type calendarsResponse struct {
	CanEdit  bool   `json:"canEdit"`
	CanShare bool   `json:"canShare"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Owner    struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	} `json:"owner"`
}

func (s *api) GetSharedCalendars() (ppCals []*calendar.Calendar, _ error) {
	return s.getCalendars(false)
}
func (s *api) GetMainCalendars() (ppCals []*calendar.Calendar, _ error) {
	return s.getCalendars(true)
}

func (s *api) getCalendars(maincal bool) (ppCals []*calendar.Calendar, _ error) {
	params := queryParams{
		"$select": strings.Join([]string{"canEdit", "canShare", "name"}, ","),
	}
	var rawResponse []struct {
		Value []calendarsResponse `json:"value"`
	}
	err := s.get("/me/calendars", params, &rawResponse)
	if err != nil {
		return nil, err
	}
	for _, res := range rawResponse {
		for _, raw := range res.Value {
			if raw.CanEdit == true && raw.CanShare == maincal {
				desc := raw.Owner.Address
				if desc == "" {
					desc = raw.Owner.Name
				}
				ppCals = append(ppCals, &calendar.Calendar{
					Active:      true,
					CustomerID:  s.customerID,
					Name:        raw.Name,
					Description: desc,
					RefType:     s.refType,
					RefID:       raw.ID,
					UserRefID:   s.ids.CalendarUserRefID(raw.Owner.Address),
				})
			}
		}
	}
	return

}
