package mutate

type IssueTransition struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Fields []IssueTransitionField `json:"fields"`
}

type IssueTransitionField struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	AllowedValues []AllowedValue `json:"allowed_values"`
	Required      bool           `json:"required"`
}

type AllowedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const ErrNotFound = "not_found"
