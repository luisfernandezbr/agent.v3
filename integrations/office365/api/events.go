package api

import (
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/calendar"
)

type calendarViewResponse struct {
	Attendees []struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
		Status struct {
			Response string `json:"response"`
		} `json:"status"`
		Type string `json:"type"`
	} `json:"attendees"`
	Body struct {
		Content string `json:"content"`
	} `json:"body"`
	End struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"end"`
	ID       string `json:"id"`
	Location struct {
		Address struct {
			City            string `json:"city,omitempty"`
			CountryOrRegion string `json:"countryOrRegion,omitempty"`
			PostalCode      string `json:"postalCode,omitempty"`
			State           string `json:"state,omitempty"`
			Street          string `json:"street,omitempty"`
		} `json:"address"`
		DisplayName string `json:"displayName"`
	} `json:"location"`
	OnlineMeetingURL string `json:"onlineMeetingUrl"`
	Organizer        struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
	} `json:"organizer"`
	ResponseStatus struct {
		Response string `json:"response"`
	} `json:"responseStatus"`
	ShowAs string `json:"showAs"`
	Start  struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"start"`
	Subject string `json:"subject"`
	WebLink string `json:"WebLink"`
}

func (s *api) GetEventsAndUsers(calid string) (newEvents []*calendar.Event, allUsers map[string]*calendar.User, _ error) {

	fields := []string{
		"subject", "body", "location", "organizer", "end", "start",
		"responseStatus", "attendees", "showAs", "onlineMeetingUrl",
	}
	params := queryParams{
		"startDateTime": time.Now().AddDate(-1, 0, 0).Format(time.RFC3339Nano),
		"endDateTime":   time.Now().AddDate(1, 0, 0).Format(time.RFC3339Nano),
		"$select":       strings.Join(fields, ","),
	}
	var res []struct {
		Value []calendarViewResponse `json:"value"`
	}
	err := s.get("me/calendars/"+calid+"/calendarview", params, &res)
	if err != nil {
		return nil, nil, err
	}
	allUsers = map[string]*calendar.User{}
	for _, r := range res {
		for _, evt := range r.Value {
			newEvent := &calendar.Event{}
			newEvent.CustomerID = s.customerID
			newEvent.Name = evt.Subject
			newEvent.Description = strings.TrimSpace(strings.Replace(evt.Body.Content, "\r\n", "\n", -1))
			newEvent.RefType = s.refType
			newEvent.RefID = evt.ID
			newEvent.CalendarID = s.ids.CalendarEvent(calid)
			newEvent.Location.URL = evt.OnlineMeetingURL
			newEvent.Location.Name = evt.Location.DisplayName
			newEvent.Location.Details = pjson.Stringify(evt.Location.Address)
			newEvent.Busy = evt.ShowAs == "busy"
			newEvent.OwnerRefID = s.ids.CalendarUserRefID(evt.Organizer.EmailAddress.Address)
			switch strings.ToLower(evt.ResponseStatus.Response) {
			case "accepted", "organizer":
				newEvent.Status = calendar.EventStatusConfirmed
			case "tentativelyaccepted":
				newEvent.Status = calendar.EventStatusTentative
			case "declined":
				newEvent.Status = calendar.EventStatusCancelled
			default:
				newEvent.Status = calendar.EventStatusTentative
			}
			for _, att := range evt.Attendees {
				var user calendar.EventParticipants
				switch strings.ToLower(att.Status.Response) {
				case "accepted", "organizer":
					user.Status = calendar.EventParticipantsStatusGoing
				case "tentativelyaccepted":
					user.Status = calendar.EventParticipantsStatusMaybe
				case "declined":
					user.Status = calendar.EventParticipantsStatusNotGoing
				default:
					user.Status = calendar.EventParticipantsStatusUnknown
				}
				refid := s.ids.CalendarUserRefID(att.EmailAddress.Address)
				user.UserRefID = refid
				newEvent.Participants = append(newEvent.Participants, user)

				allUsers[refid] = &calendar.User{
					CustomerID: s.customerID,
					Email:      att.EmailAddress.Address,
					Name:       att.EmailAddress.Name,
					RefID:      refid,
					RefType:    s.refType,
				}
			}
			var parsed time.Time
			var err error
			if parsed, err = convertDate(evt.Start.DateTime, evt.Start.TimeZone); err != nil {
				s.logger.Error("could not figure our start time", "err", err)
				continue
			}
			date.ConvertToModel(parsed, &newEvent.StartDate)

			if parsed, err = convertDate(evt.End.DateTime, evt.End.TimeZone); err != nil {
				s.logger.Error("could not figure our end time", "err", err)
				continue
			}
			date.ConvertToModel(parsed, &newEvent.EndDate)
			newEvents = append(newEvents, newEvent)
		}
	}
	return
}

func convertDate(str string, tz string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, str+"Z")
	if err != nil {
		return parsed, err
	}
	timezone, err := time.LoadLocation(tz)
	if err == nil {
		parsed = parsed.In(timezone)
	}

	return parsed, nil
}
