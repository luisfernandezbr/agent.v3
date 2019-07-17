package api

type User struct {
	Self         string  `json:"self"`
	Name         string  `json:"name"`
	Key          string  `json:"key"`
	AccountID    string  `json:"accountId"`
	EmailAddress string  `json:"emailAddress"`
	Avatars      Avatars `json:"avatarUrls"`
	DisplayName  string  `json:"displayName"`
	Active       bool    `json:"active"`
	Timezone     string  `json:"timeZone"`
	AccountType  string  `json:"accountType"`
}

// Avatars is a type that describes a set of avatar image properties
type Avatars struct {
	XSmall string `json:"16x16"`
	Small  string `json:"24x24"`
	Medium string `json:"32x32"`
	Large  string `json:"48x48"`
}
