package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSprints(t *testing.T) {
	sprints := NewSprints()

	assert := assert.New(t)

	{

		data := `com.atlassian.greenhopper.service.sprint.Sprint@50a49961[id=1,rapidViewId=1,state=ACTIVE,name=my Sprint 1,goal=,startDate=2019-07-17T14:57:50.444Z,endDate=2019-07-31T14:57:00.000Z,completeDate=<null>,sequence=1]`

		err := sprints.processIssueSprint("issue1", data)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(Sprint{
			ID:            1,
			OriginBoardID: 1,
			Name:          "my Sprint 1",
			Goal:          "",
			State:         "ACTIVE",
			StartDate:     "2019-07-17T14:57:50.444Z",
			EndDate:       "2019-07-31T14:57:00.000Z",
			CompleteDate:  "",
			Issues:        map[string]bool{"issue1": true},
		}, *sprints.data[1])
	}
	{
		data := `com.atlassian.greenhopper.service.sprint.Sprint@50a49961[id=1,rapidViewId=1,state=ACTIVE,name=my Sprint 1,goal=,startDate=2019-07-17T14:57:50.444Z,endDate=2019-07-31T14:57:00.000Z,completeDate=<null>,sequence=1]`

		err := sprints.processIssueSprint("issue2", data)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(map[string]bool{
			"issue1": true,
			"issue2": true,
		}, sprints.data[1].Issues)
	}

}
