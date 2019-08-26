package jiracommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSprints(t *testing.T) {

	cases := []struct {
		Label string
		Data  string
		Want  []Sprint
	}{
		{
			"empty",
			"",
			nil,
		},
		{
			`one_record`,
			`["com.atlassian.greenhopper.service.sprint.Sprint@50a49961[id=1,rapidViewId=1,state=ACTIVE,name=my Sprint 1,goal=,startDate=2019-07-17T14:57:50.444Z,endDate=2019-07-31T14:57:00.000Z,completeDate=<null>,sequence=1]"]`,
			[]Sprint{
				{
					ID:            1,
					OriginBoardID: 1,
					Name:          "my Sprint 1",
					Goal:          "",
					State:         "ACTIVE",
					StartDate:     "2019-07-17T14:57:50.444Z",
					EndDate:       "2019-07-31T14:57:00.000Z",
					CompleteDate:  "",
				},
			},
		},
		{
			`multiple_records`,
			`["com.atlassian.greenhopper.service.sprint.Sprint@7a8912f9[id=1,rapidViewId=1,state=ACTIVE,name=Sample Sprint 2,startDate=2019-07-15T07:57:56.277Z,endDate=2019-07-29T08:17:56.277Z,completeDate=\u003cnull\u003e,sequence=1,goal=\u003cnull\u003e]","com.atlassian.greenhopper.service.sprint.Sprint@3a58188b[id=2,rapidViewId=1,state=CLOSED,name=Sample Sprint 1,startDate=2019-07-01T06:47:58.074Z,endDate=2019-07-15T06:47:58.075Z,completeDate=2019-07-15T05:27:58.074Z,sequence=2,goal=\u003cnull\u003e]"]`,
			[]Sprint{
				{
					ID:            1,
					OriginBoardID: 1,
					Name:          "Sample Sprint 2",
					Goal:          "",
					State:         "ACTIVE",
					StartDate:     "2019-07-15T07:57:56.277Z",
					EndDate:       "2019-07-29T08:17:56.277Z",
					CompleteDate:  "",
				},
				{
					ID:            2,
					OriginBoardID: 1,
					Name:          "Sample Sprint 1",
					Goal:          "",
					State:         "CLOSED",
					StartDate:     "2019-07-01T06:47:58.074Z",
					EndDate:       "2019-07-15T06:47:58.075Z",
					CompleteDate:  "2019-07-15T05:27:58.074Z",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Label, func(t *testing.T) {
			got, err := parseSprints(c.Data)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.Want, got)
		})
	}
}

func TestParseSprintOne(t *testing.T) {

	cases := []struct {
		Label string
		Data  string
		Want  Sprint
	}{
		{
			`basic`,
			`com.atlassian.greenhopper.service.sprint.Sprint@50a49961[id=1,rapidViewId=1,state=ACTIVE,name=my Sprint 1,goal=,startDate=2019-07-17T14:57:50.444Z,endDate=2019-07-31T14:57:00.000Z,completeDate=<null>,sequence=1]`,
			Sprint{
				ID:            1,
				OriginBoardID: 1,
				Name:          "my Sprint 1",
				Goal:          "",
				State:         "ACTIVE",
				StartDate:     "2019-07-17T14:57:50.444Z",
				EndDate:       "2019-07-31T14:57:00.000Z",
				CompleteDate:  "",
			},
		},
		{
			// sprint name is set to "Special ,startDate=" in UI
			// not possible to parse this unambiguously
			`special1`,
			"com.atlassian.greenhopper.service.sprint.Sprint@382b1fac[id=1,rapidViewId=1,state=CLOSED,name=Special ,startDate=,startDate=2019-07-15T07:57:56.277Z,endDate=2019-07-29T08:17:56.277Z,completeDate=2019-08-26T17:10:43.425Z,sequence=1,goal=\\u003cnull\\u003e]",
			Sprint{
				ID:            1,
				OriginBoardID: 1,
				// BUG: we really wanted the below, but current parser cuts names at ","
				//Name:          "Special ,startDate=",
				Name:  "Special ",
				Goal:  "",
				State: "CLOSED",
				// BUG: we wanted the below, but because of startDate in the name it wasn't parsed properly
				//StartDate:    "2019-07-15T07:57:56.277Z",
				StartDate:    "",
				EndDate:      "2019-07-29T08:17:56.277Z",
				CompleteDate: "2019-08-26T17:10:43.425Z",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Label, func(t *testing.T) {
			got, err := parseSprint(c.Data)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.Want, got)
		})
	}
}

func TestParseSprintsErrors(t *testing.T) {

	cases := []struct {
		Label string
		Data  string
	}{
		{
			`invalid_json`,
			`x`,
		},
		{
			`no_fields`,
			`["com.atlassian.greenhopper.service.sprint.Sprint@50a49961[]"]`,
		},
		{
			`unclosed`,
			`["com.atlassian.greenhopper.service.sprint.Sprint@50a49961["]`,
		},
		{
			`id_not_int`,
			`["com.atlassian.greenhopper.service.sprint.Sprint@50a49961[id=x]"]`,
		},
	}

	for _, c := range cases {
		t.Run(c.Label, func(t *testing.T) {
			_, err := parseSprints(c.Data)
			if err == nil {
				t.Fatal("wanted an error")
			} else {
				t.Log(err)
			}
		})
	}
}
