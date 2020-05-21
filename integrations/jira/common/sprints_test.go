package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testParsePrintTime(t *testing.T, ts string) time.Time {
	t.Helper()
	got, err := parseSprintTime(ts)
	if err != nil {
		t.Fatal(err)
	}
	return got

}

func TestParseSprints(t *testing.T) {

	parseTime := testParsePrintTime

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
					StartDate:     parseTime(t, "2019-07-17T14:57:50.444Z"),
					EndDate:       parseTime(t, "2019-07-31T14:57:00.000Z"),
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
					StartDate:     parseTime(t, "2019-07-15T07:57:56.277Z"),
					EndDate:       parseTime(t, "2019-07-29T08:17:56.277Z"),
				},
				{
					ID:            2,
					OriginBoardID: 1,
					Name:          "Sample Sprint 1",
					Goal:          "",
					State:         "CLOSED",
					StartDate:     parseTime(t, "2019-07-01T06:47:58.074Z"),
					EndDate:       parseTime(t, "2019-07-15T06:47:58.075Z"),
					CompleteDate:  parseTime(t, "2019-07-15T05:27:58.074Z"),
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

	parseTime := testParsePrintTime

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
				StartDate:     parseTime(t, "2019-07-17T14:57:50.444Z"),
				EndDate:       parseTime(t, "2019-07-31T14:57:00.000Z"),
			},
		},
		{
			`goal`,
			`com.atlassian.greenhopper.service.sprint.Sprint@4cfd845f[id=15,rapidViewId=1,state=CLOSED,name=FE Sprint 6,startDate=2017-10-30T16:00:59.035Z,endDate=2017-11-13T17:00:00.000Z,completeDate=2017-11-13T21:36:59.267Z,sequence=15,goal=Build out secondary views and continue performance and UI improvements.]`,
			Sprint{
				ID:            15,
				OriginBoardID: 1,
				Name:          "FE Sprint 6",
				Goal:          "Build out secondary views and continue performance and UI improvements.",
				State:         "CLOSED",
				StartDate:     parseTime(t, "2017-10-30T16:00:59.035Z"),
				EndDate:       parseTime(t, "2017-11-13T17:00:00.000Z"),
				CompleteDate:  parseTime(t, "2017-11-13T21:36:59.267Z"),
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
				EndDate:      parseTime(t, "2019-07-29T08:17:56.277Z"),
				CompleteDate: parseTime(t, "2019-08-26T17:10:43.425Z"),
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

func TestParseSprintTime(t *testing.T) {
	cases := []struct {
		In   string
		Want string
	}{
		{
			"2019-08-26T17:10:43.425Z",
			"2019-08-26T17:10:43.425Z",
		},
		{
			"2019-05-06T10:14:52.527-04:00",
			"2019-05-06T10:14:52.527-04:00",
		},
	}
	for _, c := range cases {
		got, err := parseSprintTime(c.In)
		if err != nil {
			t.Fatal(err)
		}
		want, err := time.Parse(time.RFC3339, c.Want)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(want) {
			t.Errorf("wanted time parsed as %v, got %v", want, got)
		}
	}
}

func TestParseSprintTimeEmpty(t *testing.T) {
	res, err := parseSprintTime("")
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsZero() {
		t.Fatal("wanted zero time")
	}
}
