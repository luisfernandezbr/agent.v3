package jiracommonapi

type User struct {
	// AccountID not available in hosted jira.
	AccountID    string  `json:"accountId"`
	Self         string  `json:"self"`
	Name         string  `json:"name"`
	Key          string  `json:"key"`
	EmailAddress string  `json:"emailAddress"`
	Avatars      Avatars `json:"avatarUrls"`
	DisplayName  string  `json:"displayName"`
	Active       bool    `json:"active"`
	Timezone     string  `json:"timeZone"`

	Groups struct {
		Groups []UserGroup `json:"items,omitempty"`
	} `json:"groups"`
}

type UserGroup struct {
	Name string `json:"name,omitempty"`
}

func (s User) IsZero() bool {
	return s.RefID() == ""
}

func (s User) RefID() string {
	if s.AccountID != "" {
		return s.AccountID
	}
	return s.Key
}

// Avatars is a type that describes a set of avatar image properties
type Avatars struct {
	XSmall string `json:"16x16"`
	Small  string `json:"24x24"`
	Medium string `json:"32x32"`
	Large  string `json:"48x48"`
}
