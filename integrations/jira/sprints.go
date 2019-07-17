package main

import (
	"fmt"
	"regexp"
	"strconv"
)

type Sprints struct {
	data map[int]*Sprint
}

func NewSprints() *Sprints {
	s := &Sprints{}
	s.data = map[int]*Sprint{}
	return s
}

type Sprint struct {
	ID            int
	Self          string
	Name          string
	Goal          string
	State         string
	StartDate     string
	EndDate       string
	CompleteDate  string
	OriginBoardID int
	//Fetched       string

	Issues map[string]bool
}

var idRe = regexp.MustCompile(`id=(\d+)`)
var boardIDRe = regexp.MustCompile(`rapidViewId=([^,\]]*)`)
var stateRe = regexp.MustCompile(`state=([^\]=]*)[,\]]`)
var nameRe = regexp.MustCompile(`name=([^\]=]*)[,\]]`)
var goalRe = regexp.MustCompile(`goal=([^\]=]*)[,\]]`)
var startDateRe = regexp.MustCompile(`startDate=([^\]=]*)[,\]]`)
var completeDateRe = regexp.MustCompile(`completeDate=([^\]=]*)[,\]]`)
var endDateRe = regexp.MustCompile(`endDate=([^\]=]*)[,\]]`)

var sprintRE = regexp.MustCompile(`com.atlassian.greenhopper.service.sprint.Sprint@.{0,8}\[([^\]]*)\]`)

func processNull(val string) string {
	if val == "<null>" {
		return ""
	}
	if val == "\\u003cnull\\u003e" {
		return ""
	}
	return val
}

// not safe for concurrent use
func (s *Sprints) processIssueSprint(issueid string, value string) error {
	// NOTE: this is a bit of hack since the current 3LO OAuth API from JIRA only supports JIRA cloud API
	// and not the Agile API which we need to get boards and issues for those boards. For now, we're going
	// to just take this information directly from the issues to make it look like what the API can give us.
	// the biggest limitation is that we can only get sprints, not kanban boards
	sprintsArr := sprintRE.FindAllStringSubmatch(value, -1)
	if len(sprintsArr) < 1 {
		return fmt.Errorf("sprint field value does not contain data we expect, issue: %v data: %v", issueid, value)
	}

	for _, value := range sprintsArr {
		sprintid0, err := strconv.ParseInt(idRe.FindStringSubmatch(value[0])[1], 10, 32)
		if err != nil {
			panic(fmt.Errorf("Error processing <<%s>>", value))
		}
		sprintid := int(sprintid0)
		sprint, ok := s.data[sprintid]
		if ok {
			// was already added, mark the issue
			sprint.Issues[issueid] = true
			return nil
		}

		var boardid int64
		boardIDStr := processNull(boardIDRe.FindStringSubmatch(value[0])[1])
		if boardIDStr == "" {
			boardid = 0
		} else {
			boardid, err = strconv.ParseInt(boardIDStr, 10, 32)
			if err != nil {
				panic(fmt.Errorf("Error processing <<%s>>", value))
			}
		}
		state := stateRe.FindStringSubmatch(value[0])[1]
		name := nameRe.FindStringSubmatch(value[0])[1]
		goal := processNull(goalRe.FindStringSubmatch(value[0])[1])
		startDate := processNull(startDateRe.FindStringSubmatch(value[0])[1])
		endDate := processNull(endDateRe.FindStringSubmatch(value[0])[1])
		completedDate := processNull(completeDateRe.FindStringSubmatch(value[0])[1])
		sprint = &Sprint{
			ID:            int(sprintid),
			OriginBoardID: int(boardid),
			Name:          name,
			Goal:          goal,
			State:         state,
			StartDate:     startDate,
			EndDate:       endDate,
			CompleteDate:  completedDate,
			Issues:        map[string]bool{issueid: true},
			//Fetched:       time.Now().Format("2006-01-02T15:04:05.000000Z-07:00"),
		}

		s.data[sprintid] = sprint
	}
	return nil
}
