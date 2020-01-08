package jiracommonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSprintRegexp(t *testing.T) {

	gooddata := `com.atlassian.greenhopper.service.sprint.Sprint@75abc849[id=3,rapidViewId=6,state=ACTIVE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3`
	baddata1 := `com.atlassian.greenhopper.service.sprint.Sprint@75abc849[id=3,rapidViewId=6,state=COMPLETE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3`
	baddata2 := `com.atlassian.greenhopper.service.sprints.Sprint@75abc849[id=3,rapidViewId=6,state=ACTIVE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3`

	good := sprintRegexp.FindAllStringSubmatch(gooddata, -1)
	bad1 := sprintRegexp.FindAllStringSubmatch(baddata1, -1)
	bad2 := sprintRegexp.FindAllStringSubmatch(baddata2, -1)

	assert.Equal(t, len(good), 1)
	assert.Equal(t, len(good[0]), 4)
	assert.Equal(t, good[0][2], "3")

	assert.Equal(t, len(bad1), 0)
	assert.Equal(t, len(bad2), 0)
}
