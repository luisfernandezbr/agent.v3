package jiracommon

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NOTE: this is a bit of hack since the current 3LO OAuth API from JIRA only supports JIRA cloud API
// and not the Agile API which we need to get boards and issues for those boards. For now, we're going
// to just take this information directly from the issues to make it look like what the API can give us.
// the biggest limitation is that we can only get sprints, not kanban boards

type Sprints struct {
	// map[sprintID]Sprint
	sprints map[int]Sprint
	// map[sprintID]map[issueID]exists
	sprintIssues map[int]map[string]bool
}

func NewSprints() *Sprints {
	s := &Sprints{}
	s.sprints = map[int]Sprint{}
	s.sprintIssues = map[int]map[string]bool{}
	return s
}

// not safe for concurrent use
func (s *Sprints) processIssueSprint(issueID string, value string) error {
	if value == "" {
		return nil
	}
	data, err := parseSprints(value)
	if err != nil {
		return err
	}
	for _, sp := range data {
		if _, ok := s.sprints[sp.ID]; !ok {
			s.sprints[sp.ID] = sp
		}
		if _, ok := s.sprintIssues[sp.ID]; !ok {
			s.sprintIssues[sp.ID] = map[string]bool{}
		}
		s.sprintIssues[sp.ID][issueID] = true
	}
	return nil
}

func (s *Sprints) SprintsWithIssues() (res []SprintWithIssues) {
	for id, sp := range s.sprints {
		one := SprintWithIssues{}
		one.Sprint = sp
		if issues, ok := s.sprintIssues[id]; ok {
			for issueID := range issues {
				one.Issues = append(one.Issues, issueID)
			}
		}
		res = append(res, one)
	}
	return
}

type SprintWithIssues struct {
	Sprint
	Issues []string
}

type Sprint struct {
	ID            int
	Name          string
	Goal          string
	State         string
	StartDate     time.Time
	EndDate       time.Time
	CompleteDate  time.Time
	OriginBoardID int
}

func parseSprints(data string) (res []Sprint, _ error) {
	if data == "" {
		return nil, nil
	}
	var values []string
	err := json.Unmarshal([]byte(data), &values)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		s, err := parseSprint(v)
		if err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return
}

func parseSprint(data string) (res Sprint, _ error) {
	m, err := parseSprintIntoKV(data)
	if err != nil {
		return res, err
	}
	for k := range m {
		m[k] = processNull(m[k])
	}
	if m["id"] != "" {
		res.ID, err = strconv.Atoi(m["id"])
		if err != nil {
			return res, fmt.Errorf("can't parse id field %v", err)
		}
	}
	res.Name = m["name"]
	res.Goal = m["goal"]
	res.State = m["state"]
	res.StartDate, err = parseSprintTime(m["startDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse startDate %v", err)
	}
	res.EndDate, err = parseSprintTime(m["endDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse endDate %v", err)
	}
	res.CompleteDate, err = parseSprintTime(m["completeDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse completeDate %v", err)
	}
	if m["rapidViewId"] != "" {
		res.OriginBoardID, err = strconv.Atoi(m["rapidViewId"])
		if err != nil {
			return res, fmt.Errorf("can't parse rapidViewId field %v", err)
		}
	}
	return
}

func processNull(val string) string {
	if val == "<null>" {
		return ""
	}
	if val == "\\u003cnull\\u003e" {
		return ""
	}
	return val
}

func parseSprintIntoKV(data string) (map[string]string, error) {
	res := map[string]string{}
	i := strings.Index(data, "[")
	if i == 0 {
		return res, errors.New("can't find [")
	}
	fields := strings.TrimSuffix(data[i+1:], "]")
	if len(fields) == 0 {
		return res, errors.New("no fields")
	}
	for f, re := range sprintREs {
		v := re.FindStringSubmatch(fields)
		if len(v) != 2 {
			continue
		}
		res[f] = v[1]
	}
	return res, nil
}

var sprintREs map[string]*regexp.Regexp

func init() {
	sprintREs = map[string]*regexp.Regexp{}
	for _, f := range []string{"id", "rapidViewId", "state", "name", "startDate", "endDate", "completeDate", "sequence", "goal"} {
		sprintREs[f] = regexp.MustCompile(f + "=([^,]*)")
	}
}

func parseSprintTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, ts)
}
