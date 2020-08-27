package api

import (
	"net/url"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/calendar"
)

// GetCalendar returns all the events from a specifc calendar
func (s *api) GetEventsAndUsers(calid string) (res []*calendar.Event, allUsers map[string]*calendar.User, err error) {

	params := queryParams{
		"maxResults":   "2500",
		"showDeleted":  "true",
		"singleEvents": "true",
		"timeMin":      time.Now().Format(time.RFC3339),
		"timeMax":      time.Now().AddDate(0, 1, 0).Format(time.RFC3339),
	}
	var events []EventObjectRaw
	err = s.get("calendars/"+url.QueryEscape(calid)+"/events", params, &events)
	if err != nil {
		return
	}
	allUsers = map[string]*calendar.User{}
	for _, each := range events {
		for _, evt := range each.Items {
			newEvent := &calendar.Event{}
			newEvent.CustomerID = s.customerID
			newEvent.Name = evt.Summary
			newEvent.Description = evt.Description
			newEvent.RefType = s.refType
			newEvent.RefID = evt.ID
			newEvent.CalendarID = s.ids.CalendarCalendar(s.ids.CalendarCalendarRefID(calid))
			newEvent.AttendeeRefID = s.ids.CalendarUserRefID(calid)
			newEvent.Location.URL = evt.Location
			newEvent.OwnerRefID = s.ids.CalendarUserRefID(evt.Organizer.Email)

			if evt.Status == "cancelled" {
				newEvent.Status = calendar.EventStatusCancelled
			} else {
				for _, att := range evt.Attendees {
					if att.Email == calid {

						allUsers[newEvent.AttendeeRefID] = &calendar.User{
							CustomerID: s.customerID,
							Email:      att.Email,
							Name:       att.DisplayName,
							RefID:      newEvent.AttendeeRefID,
							RefType:    s.refType,
						}
						switch att.ResponseStatus {
						case "accepted":
							newEvent.Status = calendar.EventStatusConfirmed
						case "needsAction", "tentative":
							newEvent.Status = calendar.EventStatusTentative
						case "declined":
							newEvent.Status = calendar.EventStatusCancelled
						}
						break
					}
				}
			}
			if newEvent.Status != calendar.EventStatusCancelled {
				// cancelled events don't have a start and end date
				// the pipeline should skip these events if they don't exist in historical
				// or update them to "cancelled" if they exist in historical
				startDate := evt.Start.DateTime
				endDate := evt.End.DateTime
				if startDate.IsZero() && evt.Start.Date != "" {
					startDate, err = dateStringToTime(evt.Start.Date, each.TimeZone)
					if err != nil {
						s.logger.Error("error getting start date, skipping event", "err", err)
						continue
					}
				}
				if endDate.IsZero() && evt.End.Date != "" {
					endDate, err = dateStringToTime(evt.End.Date, each.TimeZone)
					if err != nil {
						s.logger.Error("error getting end date, skipping event", "err", err)
						continue
					}
				}
				if startDate.IsZero() {
					s.logger.Error("start date is zero, skipping event")
					continue
				}
				if endDate.IsZero() {
					s.logger.Error("end date is zero, skipping event")
					continue
				}
				date.ConvertToModel(startDate, &newEvent.StartDate)
				date.ConvertToModel(endDate, &newEvent.EndDate)
			}
			res = append(res, newEvent)
		}
	}
	return
}

func dateStringToTime(d string, tz string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", d)
	if err != nil {
		return time.Time{}, err
	}
	timezone, err := time.LoadLocation(tz)
	if err == nil {
		parsed = parsed.In(timezone)
	}
	return parsed, nil
}
