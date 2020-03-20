package jiracommonapi

import "github.com/pinpt/integration-sdk/work"

func getMappedIssueType(name string, subtask bool) work.IssueTypeMappedType {
	if subtask {
		// any subtask will have this flag set
		return work.IssueTypeMappedTypeSubtask
	}
	// map out of the box jira types that are known
	switch name {
	case "Story":
		return work.IssueTypeMappedTypeStory
	case "Improvement", "Enhancement":
		return work.IssueTypeMappedTypeEnhancement
	case "Epic":
		return work.IssueTypeMappedTypeEpic
	case "New Feature":
		return work.IssueTypeMappedTypeFeature
	case "Bug":
		return work.IssueTypeMappedTypeBug
	case "Task":
		return work.IssueTypeMappedTypeTask
	}
	// otherwise this is a custom type which can be mapped from the app side by the user
	return work.IssueTypeMappedTypeUnknown
}

func IssueTypes(qc QueryContext) (res []work.IssueType, rerr error) {

	objectPath := "issuetype"

	var issueTypes []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"iconUrl"`
		Subtask     bool   `json:"subtask"`
	}

	err := qc.Req.Get(objectPath, nil, &issueTypes)
	if err != nil {
		rerr = err
		return
	}

	found := make(map[string]bool)

	for _, val := range issueTypes {
		if !found[val.Name] {
			found[val.Name] = true // can have duplicates scoped by project id which we might want to support in the future
			res = append(res, work.IssueType{
				ID:          work.NewIssueTypeID(qc.CustomerID, "jira", val.ID),
				CustomerID:  qc.CustomerID,
				Name:        val.Name,
				Description: &val.Description,
				IconURL:     &val.Icon,
				MappedType:  getMappedIssueType(val.Name, val.Subtask),
				RefType:     "jira",
				RefID:       val.ID,
			})
		}
	}

	return
}
