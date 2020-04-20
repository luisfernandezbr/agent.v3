package api

import (
	"fmt"

	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/calendar"
)

// GetCalendar returns all the calendar that the logged in use has subcribed to
func (s *api) GetCalendars() (res []*calendar.Calendar, err error) {
	var cals []CalendarsObjectRaw
	err = s.get("users/me/calendarList", queryParams{
		"maxResults":    "250",
		"minAccessRole": "writer",
	}, &cals)
	if err != nil {
		return
	}
	for _, c := range cals {
		for _, item := range c.Items {
			res = append(res, &calendar.Calendar{
				CustomerID:  s.customerID,
				Name:        item.Summary,
				Description: item.ID, // use the email (which is id) for description
				RefType:     s.refType,
				RefID:       item.ID,
				UserRefID:   s.ids.CalendarUserID(item.ID),
			})
		}
	}
	return
}

// GetCalendar returns calendar information from a user
func (s *api) GetCalendar(calID string) (res *calendar.Calendar, err error) {
	var raw []CalendarObjectRaw
	err = s.get("calendars/"+calID, queryParams{}, &raw)
	if len(raw) != 1 {
		return nil, fmt.Errorf("return 0 or more than 1 calendar. %v", pjson.Stringify(raw))
	}
	res = &calendar.Calendar{
		CustomerID:  s.customerID,
		Name:        raw[0].Summary,
		Description: raw[0].ID, // use the email (which is id) for description
		RefType:     s.refType,
		RefID:       raw[0].ID,
		UserRefID:   s.ids.CalendarUserID(raw[0].ID),
	}
	return
}
