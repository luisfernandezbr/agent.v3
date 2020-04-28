package api

import (
	"net/url"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/calendar"
)

// GetCalendar returns all the events from a specifc calendar
func (s *api) GetEventsAndUsers(calid string, syncToken string) (res []*calendar.Event, allUsers map[string]*calendar.User, newToken string, err error) {

	params := queryParams{
		"maxResults": "2500",
	}
	if syncToken != "" {
		params["syncToken"] = syncToken
	}
	var events []EventObjectRaw
	err = s.get("calendars/"+url.QueryEscape(calid)+"/events", params, &events)
	if err != nil {
		return
	}
	refType := s.refType

	allUsers = make(map[string]*calendar.User)
	for _, each := range events {
		newToken = each.NextSyncToken
		for _, evt := range each.Items {
			newEvent := &calendar.Event{}
			newEvent.CustomerID = s.customerID
			newEvent.Name = evt.Summary
			newEvent.Description = evt.Description
			newEvent.RefType = refType
			newEvent.RefID = evt.ID
			newEvent.CalendarID = s.ids.CalendarCalendar(s.ids.CalendarCalendarRefID(calid))
			newEvent.Location.URL = evt.Location
			newEvent.OwnerRefID = s.ids.CalendarUserRefID(evt.Organizer.Email)
			switch evt.Status {
			case "confirmed":
				newEvent.Status = calendar.EventStatusConfirmed
			case "tentative":
				newEvent.Status = calendar.EventStatusTentative
			case "cancelled":
				newEvent.Status = calendar.EventStatusCancelled
			}

			for _, att := range evt.Attendees {
				refid := s.ids.CalendarUserRefID(att.Email)
				newEvent.Busy = att.ResponseStatus == "accepted"

				var user calendar.EventParticipants
				switch att.ResponseStatus {
				case "accepted":
					user.Status = calendar.EventParticipantsStatusGoing
				case "declined":
					user.Status = calendar.EventParticipantsStatusNotGoing
				case "tentative":
					user.Status = calendar.EventParticipantsStatusMaybe
				case "needsAction":
					user.Status = calendar.EventParticipantsStatusUnknown
				}
				user.UserRefID = refid
				newEvent.Participants = append(newEvent.Participants, user)

				allUsers[refid] = &calendar.User{
					CustomerID: s.customerID,
					Email:      att.Email,
					Name:       att.DisplayName,
					RefID:      refid,
					RefType:    s.refType,
				}
			}

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
