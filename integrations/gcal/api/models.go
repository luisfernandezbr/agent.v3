package api

import "time"

// EventObjectRaw response object from api
type EventObjectRaw struct {
	TimeZone      string `json:"timeZone"`
	NextSyncToken string `json:"nextSyncToken"`
	Items         []struct {
		Attendees []struct {
			DisplayName    string `json:"displayName,omitempty"`
			Email          string `json:"email"`
			Organizer      bool   `json:"organizer,omitempty"`
			ResponseStatus string `json:"responseStatus"`
			Self           bool   `json:"self,omitempty"`
		} `json:"attendees"`
		ConferenceData struct {
			ConferenceID       string `json:"conferenceId"`
			ConferenceSolution struct {
				IconURI string `json:"iconUri"`
				Key     struct {
					Type string `json:"type"`
				} `json:"key"`
				Name string `json:"name"`
			} `json:"conferenceSolution"`
			EntryPoints []struct {
				EntryPointType string `json:"entryPointType"`
				Label          string `json:"label"`
				URI            string `json:"uri"`
				Pin            string `json:"pin,omitempty"`
				RegionCode     string `json:"regionCode,omitempty"`
			} `json:"entryPoints"`
			Signature string `json:"signature"`
		} `json:"conferenceData"`
		Created time.Time `json:"created"`
		Creator struct {
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"creator"`
		Description string `json:"description"`
		End         struct {
			DateTime time.Time `json:"dateTime"`
			Date     string    `json:"date"`
		} `json:"end"`
		Etag        string `json:"etag"`
		HangoutLink string `json:"hangoutLink"`
		HTMLLink    string `json:"htmlLink"`
		ICalUID     string `json:"iCalUID"`
		ID          string `json:"id"`
		Kind        string `json:"kind"`
		Location    string `json:"location"`
		Organizer   struct {
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"organizer"`
		Reminders struct {
			UseDefault bool `json:"useDefault"`
		} `json:"reminders"`
		Sequence int `json:"sequence"`
		Start    struct {
			DateTime time.Time `json:"dateTime"`
			Date     string    `json:"date"`
		} `json:"start"`
		Status  string    `json:"status"`
		Summary string    `json:"summary"`
		Updated time.Time `json:"updated"`
	}
}

// CalendarObjectRaw response object from api
type CalendarObjectRaw struct {
	Kind                 string `json:"kind"`
	Etag                 string `json:"etag"`
	ID                   string `json:"id"`
	Summary              string `json:"summary"`
	TimeZone             string `json:"timeZone"`
	Location             string `json:"location"`
	ConferenceProperties struct {
		AllowedConferenceSolutionTypes []string `json:"allowedConferenceSolutionTypes"`
	} `json:"conferenceProperties"`
}

// CalendarsObjectRaw response object from api
type CalendarsObjectRaw struct {
	Items []struct {
		Kind             string `json:"kind"`
		Etag             string `json:"etag"`
		ID               string `json:"id"`
		Summary          string `json:"summary"`
		TimeZone         string `json:"timeZone"`
		ColorID          string `json:"colorId"`
		BackgroundColor  string `json:"backgroundColor"`
		ForegroundColor  string `json:"foregroundColor"`
		Selected         bool   `json:"selected"`
		AccessRole       string `json:"accessRole"`
		DefaultReminders []struct {
			Method  string `json:"method"`
			Minutes int    `json:"minutes"`
		} `json:"defaultReminders"`
		NotificationSettings struct {
			Notifications []struct {
				Type   string `json:"type"`
				Method string `json:"method"`
			} `json:"notifications"`
		} `json:"notificationSettings"`
		Primary              bool `json:"primary"`
		ConferenceProperties struct {
			AllowedConferenceSolutionTypes []string `json:"allowedConferenceSolutionTypes"`
		} `json:"conferenceProperties"`
	} `json:"items"`
}
