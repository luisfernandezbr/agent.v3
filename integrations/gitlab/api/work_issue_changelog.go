package api

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/work"
)

func WorkIssuesDiscussionsPage(qc QueryContext, projectID string, issueID string, usermap UsernameMap, params url.Values) (pi PageInfo, changelogs []work.IssueChangeLog, comments []work.IssueComment, err error) {

	qc.Logger.Debug("work issues changelog", "project", projectID)
	params.Set("notes_filter", "0")
	params.Set("persist_filter", "true")
	objectPath := pstrings.JoinURL("projects", url.QueryEscape(projectID), "issues", issueID, "discussions.json")

	params.Set("scope", "all")
	var notes []DiscussionModel
	pi, err = qc.Request(objectPath, params, &notes)
	if err != nil {
		return
	}

	for _, n := range notes {
		for _, nn := range n.Notes {
			if !nn.System {
				comment := work.IssueComment{
					RefID:     fmt.Sprint(nn.ID),
					RefType:   "gitlab",
					UserRefID: usermap[nn.Author.Username],
					IssueID:   issueID,
					ProjectID: projectID,
					Body:      nn.Body,
				}
				date.ConvertToModel(nn.CreatedAt, &comment.CreatedDate)
				date.ConvertToModel(nn.UpdatedAt, &comment.UpdatedDate)
				comments = append(comments, comment)
				continue
			}
			if nn.Body == "changed the description" {
				continue
			}
			changelog := work.IssueChangeLog{
				RefID:  fmt.Sprint(nn.ID),
				UserID: usermap[nn.Author.Username],
			}
			date.ConvertToModel(nn.CreatedAt, &changelog.CreatedDate)

			if strings.HasPrefix(nn.Body, "assigned to ") {
				// IssueChangeLogFieldAssigneeRefID
				reg := regexp.MustCompile(`@(\w)+`)
				all := reg.FindAllString(nn.Body, 2)
				if len(all) == 0 {
					qc.Logger.Warn("regex failed, body was: " + nn.Body)
					continue
				}
				toUser := strings.Replace(all[0], "@", "", 1)
				toRefID := usermap[toUser]
				var fromUser string
				var fromRefID string
				if strings.HasPrefix(nn.Body, "and unassigned") {
					fromUser = strings.Replace(all[1], "@", "", 1)
					fromRefID = usermap[fromUser]
				}
				changelog.From = fromRefID
				changelog.FromString = fromUser
				changelog.To = toRefID
				changelog.ToString = toUser
				changelog.Field = work.IssueChangeLogFieldAssigneeRefID
			} else if strings.HasPrefix(nn.Body, "unassigned ") {
				reg := regexp.MustCompile(`@(\w)+`)
				all := reg.FindAllString(nn.Body, 1)
				fromUser := strings.Replace(all[0], "@", "", 1)
				fromRefID := usermap[fromUser]
				changelog.From = fromRefID
				changelog.FromString = fromUser
				changelog.Field = work.IssueChangeLogFieldAssigneeRefID
			} else if strings.HasPrefix(nn.Body, "changed due date to ") {
				// IssueChangeLogFieldDueDate
				strdate := strings.Replace(nn.Body, "changed due date to ", "", 1)
				changelog.To = strdate
				changelog.ToString = strdate
				changelog.Field = work.IssueChangeLogFieldDueDate
			} else if strings.Contains(nn.Body, " epic ") {
				// IssueChangeLogFieldEpicID
				changelog.Field = work.IssueChangeLogFieldEpicID
				if strings.HasPrefix(nn.Body, "added to ") {
					to := strings.Replace(nn.Body, "added to epic ", "", 1)
					changelog.To = to
					changelog.ToString = to
				} else if strings.HasPrefix(nn.Body, "changed epic ") {
					to := strings.Replace(nn.Body, "changed epic ", "", 1)
					changelog.To = to
					changelog.ToString = to
				} else if strings.HasPrefix(nn.Body, "removed from ") {
					from := strings.Replace(nn.Body, "removed from epic ", "", 1)
					changelog.From = from
					changelog.FromString = from
				}
			} else if strings.HasPrefix(nn.Body, "changed title") {
				// IssueChangeLogFieldTitle
				reg := regexp.MustCompile(`\*\*(.*?)\*\*`)
				all := reg.FindAllStringSubmatch(nn.Body, -1)
				if len(all) < 2 {
					qc.Logger.Warn("regex failed, body was: " + nn.Body)
					continue
				}
				from := all[0][1]
				to := all[1][1]
				changelog.From = from
				changelog.FromString = from
				changelog.To = to
				changelog.ToString = to
				changelog.Field = work.IssueChangeLogFieldTitle
			} else {
				// not found, continue
				continue
			}
			changelogs = append(changelogs, changelog)

		}
	}

	qc.Logger.Debug("work issues changelog resource_state_events", "project", projectID)

	objectPath = pstrings.JoinURL("projects", url.QueryEscape(projectID), "issues", issueID, "resource_state_events")

	var stateEvents []ResourceStateEvents
	pi, err = qc.Request(objectPath, params, &stateEvents)
	if err != nil {
		return
	}
	for _, stateEvent := range stateEvents {
		changelog := work.IssueChangeLog{
			RefID:  fmt.Sprint(stateEvent.ID),
			UserID: strconv.FormatInt(stateEvent.User.ID, 10),
		}
		date.ConvertToModel(stateEvent.CreatedAt, &changelog.CreatedDate)

		if stateEvent.State == "closed" || stateEvent.State == "reopened" {
			changelog.To = stateEvent.State
			changelog.ToString = stateEvent.State
			changelog.Field = work.IssueChangeLogFieldStatus
		}
		changelogs = append(changelogs, changelog)
	}

	return
}
