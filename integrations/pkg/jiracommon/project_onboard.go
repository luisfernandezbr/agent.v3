package jiracommon

import "time"

func ProjectIsActive(lastIssueCreated time.Time, categoryName string) bool {
	if lastIssueCreated.IsZero() {
		return false
	}
	sixMonthsAgo := lastIssueCreated.AddDate(0, -6, 0)
	// TODO: BUG: the category name should be configurable by customer
	return lastIssueCreated.After(sixMonthsAgo) && categoryName == "Development"
}
