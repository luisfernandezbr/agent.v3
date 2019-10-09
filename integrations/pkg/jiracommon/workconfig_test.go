package jiracommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExistedStatusesOnly(t *testing.T) {

	assert := assert.New(t)

	allValues := []string{"Selected for Development", "Backlog", "Validated", "Evidence Needed", "Evidence Validated", "Done", "Ready for Promotion", "Work Required", "Rework", "Closed", "To Do", "Awaiting Release", "In Testing", "Control Validation", "In Progress", "Awaiting Validation", "Work Complete", "In Review", "Validate Evidence", "On Hold", "Gathering Evidence"}
	setValues := []string{"Work Complete", "Completed", "Closed", "Done", "Fixed"}

	expected := []string{"Work Complete", "Closed", "Done"}

	actual := getExistedStatusesOnly(allValues, setValues)

	assert.Equal(expected, actual)
}
