package jiracommonapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/go-common/datetime"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/work"
)

type CustomFieldValue struct {
	ID    string
	Name  string
	Value string
}

type IssueWithCustomFields struct {
	*work.Issue
	CustomFields []CustomFieldValue
}

func relativeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 0 {
		return fmt.Sprintf("-%dm", h*60+m) // convert to minutes
	}
	if m == 0 {
		return "-1m" // always return at least 1m ago
	}
	return fmt.Sprintf("-%dm", m)
}

var sprintRegexp = regexp.MustCompile(`(.+?sprint\.Sprint@.+?\[id=)(\d+)(,.+?state=ACTIVE.*)`)

type issueSource struct {
	ID  string `json:"id"`
	Key string `json:"key"`

	// Using map here instead of the Fields struct declared below,
	// since we extract custom fields which could have keys prefixed
	// with customfield_.
	Fields         map[string]interface{} `json:"fields"`
	RenderedFields struct {
		Description string `json:"description"`
	} `json:"renderedFields"`
	Changelog struct {
		Histories []struct {
			ID      string `json:"id"`
			Author  User   `json:"author"`
			Created string `json:"created"`
			Items   []struct {
				Field      string `json:"field"`
				FieldType  string `json:"fieldtype"`
				From       string `json:"from"`
				FromString string `json:"fromString"`
				To         string `json:"to"`
				ToString   string `json:"toString"`
			} `json:"items"`
		} `json:"histories"`
	} `json:"changelog"`
}

type linkedIssue struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type issueFields struct {
	Project struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	} `json:"project"`
	Summary  string `json:"summary"`
	DueDate  string `json:"duedate"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
	Priority struct {
		Name string `json:"name"`
	} `json:"priority"`
	IssueType struct {
		Name string `json:"name"`
	} `json:"issuetype"`
	Status struct {
		Name string `json:"name"`
	} `json:"status"`
	Resolution struct {
		Name string `json:"name"`
	} `json:"resolution"`
	Creator    User
	Reporter   User
	Assignee   User
	Labels     []string `json:"labels"`
	SprintIDs  []string
	IssueLinks []struct {
		ID   string `json:"id"`
		Type struct {
			//ID   string `json:"id"`
			Name string `json:"name"` // Using Name instead of ID for mapping
		} `json:"type"`
		OutwardIssue linkedIssue `json:"outwardIssue"`
		InwardIssue  linkedIssue `json:"inwardIssue"`
	} `json:"issuelinks"`
	Attachment []struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
		Author   struct {
			Key       string `json:"key"`
			AccountID string `json:"accountId"`
		} `json:"author"`
		Created   string `json:"created"`
		Size      int    `json:"size"`
		MimeType  string `json:"mimeType"`
		Content   string `json:"content"`
		Thumbnail string `json:"thumbnail"`
	} `json:"attachment"`
}

// IssuesAndChangelogsPage returns issues and related changelogs. Calls qc.ExportUser for each user. Current difference from jira-cloud version is that user.Key is used instead of user.AccountID everywhere.
func IssuesAndChangelogsPage(
	qc QueryContext,
	project Project,
	fieldByID map[string]CustomField,
	updatedSince time.Time,
	paginationParams url.Values) (
	pi PageInfo,
	resIssues []IssueWithCustomFields,

	rerr error) {

	objectPath := "search"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("validateQuery", "strict")
	jql := `project="` + project.JiraID + `"`

	if !updatedSince.IsZero() {
		s := relativeDuration(time.Since(updatedSince))
		jql += fmt.Sprintf(` and (created >= "%s" or updated >= "%s")`, s, s)
	}

	// CAREFUL. pipeline right now requires specific ordering for issues
	// Only needed for pipeline. Could remove otherwise.
	jql += " ORDER BY created ASC"

	params.Set("jql", jql)
	// we need both fields and renderedFields so that we can get the unprocessed (fields) and processed (html for renderedFields)
	params.Add("expand", "changelog,fields,renderedFields")
	params.Add("fields", "*navigable,attachment")

	qc.Logger.Info("issues request", "project", project.Key, "params", params)

	var rr struct {
		Total      int           `json:"total"`
		MaxResults int           `json:"maxResults"`
		Issues     []issueSource `json:"issues"`
	}

	err := qc.Req.Get(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Issues) == rr.MaxResults {
		pi.HasMore = true
	}

	for _, data := range rr.Issues {
		issue, err := convertIssue(qc, data, fieldByID)
		if err != nil {
			rerr = err
			return
		}
		resIssues = append(resIssues, issue)
	}

	return
}

// BUG: returned data will have missing start and end date, because we don't pass fieldsByID here
func IssueByID(qc QueryContext, issueIDOrKey string) (_ IssueWithCustomFields, rerr error) {
	// https://developer.atlassian.com/cloud/jira/platform/rest/v3/#api-rest-api-3-issue-issueIdOrKey-get

	objectPath := "issue/" + issueIDOrKey

	params := url.Values{}
	// we need both fields and renderedFields so that we can get the unprocessed (fields) and processed (html for renderedFields)
	params.Add("expand", "changelog,renderedFields")

	qc.Logger.Debug("issue request", "issue_id_or_key", issueIDOrKey)

	var rr issueSource

	err := qc.Req.Get(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	fieldsByID := map[string]CustomField{}
	res, err := convertIssue(qc, rr, fieldsByID)
	if err != nil {
		rerr = err
		return
	}

	return res, nil
}

func ParsePlannedDate(ts string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", ts, time.UTC)
}

// jira format: 2019-07-12T22:32:50.376+0200
const jiraTimeFormat = "2006-01-02T15:04:05.999Z0700"

func ParseTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(jiraTimeFormat, ts)
}

func convertIssue(qc QueryContext, data issueSource, fieldByID map[string]CustomField) (_ IssueWithCustomFields, rerr error) {

	var fields issueFields
	err := structmarshal.MapToStruct(data.Fields, &fields)
	if err != nil {
		rerr = err
		return
	}

	for k, v := range data.Fields {
		if strings.HasPrefix(k, "customfield_") && v != nil {
			if arr, ok := v.([]interface{}); ok {
				for _, each := range arr {
					if sprints := sprintRegexp.FindAllStringSubmatch(fmt.Sprint(each), -1); len(sprints) > 0 {
						id := sprints[0][2]
						fields.SprintIDs = append(fields.SprintIDs, qc.SprintID(id))
					}
				}
			}
		}
	}

	project := Project{}
	project.JiraID = fields.Project.ID
	project.Key = fields.Project.Key

	if project.JiraID == "" {
		panic("missing project id")
	}

	item := IssueWithCustomFields{}
	item.Issue = &work.Issue{}
	item.CustomerID = qc.CustomerID
	item.RefID = data.ID
	item.RefType = "jira"
	item.Identifier = data.Key
	item.ProjectID = qc.ProjectID(project.JiraID)
	item.SprintIds = fields.SprintIDs

	if fields.DueDate != "" {
		orig := fields.DueDate
		d, err := time.ParseInLocation("2006-01-02", orig, time.UTC)
		if err != nil {
			rerr = fmt.Errorf("could not parse duedate of jira issue: %v err: %v", orig, err)
			return
		}
		date.ConvertToModel(d, &item.DueDate)
	}

	item.Title = fields.Summary

	if data.RenderedFields.Description != "" {
		item.Description = adjustRenderedHTML(qc.WebsiteURL, data.RenderedFields.Description)
	}

	created, err := ParseTime(fields.Created)
	if err != nil {
		rerr = err
		return
	}
	date.ConvertToModel(created, &item.CreatedDate)
	updated, err := ParseTime(fields.Updated)
	if err != nil {
		rerr = err
		return
	}
	date.ConvertToModel(updated, &item.UpdatedDate)

	item.Priority = fields.Priority.Name
	item.Type = fields.IssueType.Name
	item.Status = fields.Status.Name
	item.Resolution = fields.Resolution.Name

	if !fields.Creator.IsZero() {
		item.CreatorRefID = fields.Creator.RefID()
		if qc.ExportUser != nil {
			err := qc.ExportUser(fields.Creator)
			if err != nil {
				rerr = err
				return
			}
		}
	}
	if !fields.Reporter.IsZero() {
		item.ReporterRefID = fields.Reporter.RefID()
		if qc.ExportUser != nil {
			err := qc.ExportUser(fields.Reporter)
			if err != nil {
				rerr = err
				return
			}
		}
	}
	if !fields.Assignee.IsZero() {
		item.AssigneeRefID = fields.Assignee.RefID()
		if qc.ExportUser != nil {
			err := qc.ExportUser(fields.Assignee)
			if err != nil {
				rerr = err
				return
			}
		}
	}

	item.URL = qc.IssueURL(data.Key)
	item.Tags = fields.Labels

	for _, link := range fields.IssueLinks {
		var linkType work.IssueLinkedIssuesLinkType
		reverseDirection := false
		switch link.Type.Name {
		case "Blocks":
			linkType = work.IssueLinkedIssuesLinkTypeBlocks
		case "Cloners":
			linkType = work.IssueLinkedIssuesLinkTypeClones
		case "Duplicate":
			linkType = work.IssueLinkedIssuesLinkTypeDuplicates
		case "Problem/Incident":
			linkType = work.IssueLinkedIssuesLinkTypeCauses
		case "Relates":
			linkType = work.IssueLinkedIssuesLinkTypeRelates
		default:
			qc.Logger.Error("unknown link type name", "name", link.Type.Name)
			continue
		}
		var linkedIssue linkedIssue
		if link.OutwardIssue.ID != "" {
			linkedIssue = link.OutwardIssue
		} else if link.InwardIssue.ID != "" {
			linkedIssue = link.InwardIssue
			reverseDirection = true
		} else {
			qc.Logger.Error("issue link does not have outward or inward issue", "issue_id", data.ID, "issue_key", data.Key)
			continue
		}
		link2 := work.IssueLinkedIssues{}
		link2.RefID = link.ID
		link2.IssueID = qc.IssueID(linkedIssue.ID)
		link2.IssueRefID = linkedIssue.ID
		link2.IssueIdentifier = linkedIssue.Key
		link2.ReverseDirection = reverseDirection
		link2.LinkType = linkType
		item.LinkedIssues = append(item.LinkedIssues, link2)
	}

	for _, data := range fields.Attachment {
		var attachment work.IssueAttachments
		attachment.RefID = data.ID
		attachment.Name = data.Filename
		attachment.URL = data.Content
		attachment.ThumbnailURL = data.Thumbnail
		attachment.MimeType = data.MimeType
		attachment.Size = int64(data.Size)
		user := data.Author.AccountID // cloud
		if user == "" {
			user = data.Author.Key // hosted
		}
		attachment.UserRefID = user
		created, err := ParseTime(data.Created)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(created, &attachment.CreatedDate)
		item.Attachments = append(item.Attachments, attachment)
	}

	for k, d := range data.Fields {
		if !strings.HasPrefix(k, "customfield_") {
			continue
		}

		fd, ok := fieldByID[k]
		if !ok {
			qc.Logger.Error("when processing jira issues, could not find field definition by key", "project", project.Key, "key", k)
			continue
		}
		v := ""
		if d != nil {
			ds, ok := d.(string)
			if ok {
				v = ds
			} else {
				b, err := json.Marshal(d)
				if err != nil {
					rerr = err
					return
				}
				v = string(b)
			}
		}

		if fd.Name == "Start Date" && v != "" {
			d, err := ParsePlannedDate(v)
			if err != nil {
				qc.Logger.Error("could not parse field %v as date, err: %v", fd.Name, err)
				continue
			}
			date.ConvertToModel(d, &item.PlannedStartDate)
		} else if fd.Name == "End Date" && v != "" {
			d, err := ParsePlannedDate(v)
			if err != nil {
				qc.Logger.Error("could not parse field %v as date, err: %v", fd.Name, err)
				continue
			}
			date.ConvertToModel(d, &item.PlannedEndDate)
		}

		f := CustomFieldValue{}
		f.ID = fd.ID
		f.Name = fd.Name
		f.Value = v
		item.CustomFields = append(item.CustomFields, f)
	}

	issueRefID := item.RefID

	issue := item

	// ordinal should be a monotonically increasing number for changelogs
	// the value itself doesn't matter as long as the changelog is from
	// the oldest to the newest
	//
	// Using current timestamp here instead of int, so the number is also an increasing one when running incrementals compared to previous values in the historical.
	ordinal := datetime.EpochNow()

	// Jira changelog histories are ordered from the newest to the oldest but we want changelogs to be
	// sent from the oldest event to the newest event when we send
	for i := len(data.Changelog.Histories) - 1; i >= 0; i-- {
		cl := data.Changelog.Histories[i]
		for _, data := range cl.Items {

			item := work.IssueChangeLog{}
			item.RefID = cl.ID
			item.Ordinal = ordinal

			ordinal++

			createdAt, err := ParseTime(cl.Created)
			if err != nil {
				rerr = fmt.Errorf("could not parse created time of changelog for issue: %v err: %v", issueRefID, err)
				return
			}
			date.ConvertToModel(createdAt, &item.CreatedDate)
			item.UserID = cl.Author.RefID()

			item.FromString = data.FromString + " @ " + data.From
			item.ToString = data.ToString + " @ " + data.To

			switch strings.ToLower(data.Field) {
			case "status":
				item.Field = work.IssueChangeLogFieldStatus
				item.From = data.FromString
				item.To = data.ToString
			case "resolution":
				item.Field = work.IssueChangeLogFieldResolution
				item.From = data.FromString
				item.To = data.ToString
			case "assignee":
				item.Field = work.IssueChangeLogFieldAssigneeRefID
				if data.From != "" {
					item.From = ids.WorkUserAssociatedRefID(qc.CustomerID, "jira", data.From)
				}
				if data.To != "" {
					item.To = ids.WorkUserAssociatedRefID(qc.CustomerID, "jira", data.To)
				}
			case "reporter":
				item.Field = work.IssueChangeLogFieldReporterRefID
				item.From = data.From
				item.To = data.To
			case "summary":
				item.Field = work.IssueChangeLogFieldTitle
				item.From = data.FromString
				item.To = data.ToString
			case "duedate":
				item.Field = work.IssueChangeLogFieldDueDate
				item.From = data.FromString
				item.To = data.ToString
			case "issuetype":
				item.Field = work.IssueChangeLogFieldType
				item.From = data.FromString
				item.To = data.ToString
			case "labels":
				item.Field = work.IssueChangeLogFieldTags
				item.From = data.FromString
				item.To = data.ToString
			case "priority":
				item.Field = work.IssueChangeLogFieldPriority
				item.From = data.FromString
				item.To = data.ToString
			case "project":
				item.Field = work.IssueChangeLogFieldProjectID
				if data.From != "" {
					item.From = qc.ProjectID(data.From)
				}
				if data.To != "" {
					item.From = qc.ProjectID(data.To)
				}
			case "key":
				item.Field = work.IssueChangeLogFieldIdentifier
				item.From = data.FromString
				item.To = data.ToString
			case "sprint":
				item.Field = work.IssueChangeLogFieldSprintIds
				var from, to []string
				if data.From != "" {
					for _, s := range strings.Split(data.From, ",") {
						from = append(from, work.NewSprintID(qc.CustomerID, strings.TrimSpace(s), "jira"))
					}
				}
				if data.To != "" {
					for _, s := range strings.Split(data.To, ",") {
						to = append(to, work.NewSprintID(qc.CustomerID, strings.TrimSpace(s), "jira"))
					}
				}
				item.From = strings.Join(from, ",")
				item.To = strings.Join(to, ",")
			case "parent":
				item.Field = work.IssueChangeLogFieldParentID
				if data.From != "" {
					item.From = work.NewIssueID(qc.CustomerID, "jira", data.From)
				}
				if data.To != "" {
					item.To = work.NewIssueID(qc.CustomerID, "jira", data.To)
				}
			case "epic link":
				item.Field = work.IssueChangeLogFieldEpicID
				if data.From != "" {
					item.From = work.NewIssueID(qc.CustomerID, "jira", data.From)
				}
				if data.To != "" {
					item.To = work.NewIssueID(qc.CustomerID, "jira", data.To)
				}
			default:
				// Ignore other change types
				continue
			}
			issue.ChangeLog = append(issue.ChangeLog, item)
		}

	}

	return issue, nil

}

var imgRegexp = regexp.MustCompile(`(?s)<span class="image-wrap"[^\>]*>(.*?src\=(?:\"|\')(.+?)(?:\"|\').*?)<\/span>`)

var emoticonRegexp = regexp.MustCompile(`<img class="emoticon" src="([^"]*)"[^>]*\/>`)

// we need to pull out the HTML and parse it so we can properly display it in the application. the HTML will
// have a bunch of stuff we need to cleanup for rendering in our application such as relative urls, etc. we
// clean this up here and fix any urls and weird html issues
func adjustRenderedHTML(websiteURL, data string) string {
	if data == "" {
		return ""
	}

	for _, m := range imgRegexp.FindAllStringSubmatch(data, -1) {
		url := m[2] // this is the image group
		// if a relative url but not a // meaning protocol to the page, then make an absolute url to the server
		if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
			// replace the relative url with an absolute url. the app will handle the case where the app
			// image is unreachable because the server is behind a corporate firewall and the user isn't on
			// the network when viewing it
			url = pstrings.JoinURL(websiteURL, url)
		}
		// replace the <span> wrapped thumbnail junk with just a simple image tag
		newval := strings.Replace(m[0], m[1], `<img src="`+url+`" />`, 1)
		data = strings.ReplaceAll(data, m[0], newval)
	}

	for _, m := range emoticonRegexp.FindAllStringSubmatch(data, -1) {
		url := m[1]
		if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
			url = pstrings.JoinURL(websiteURL, url)
		}
		newval := strings.Replace(m[0], m[1], url, 1)
		data = strings.ReplaceAll(data, m[0], newval)
	}

	// we apply a special tag here to allow the front-end to handle integration specific data for the integration in
	// case we need to do integration specific image handling
	return `<div class="source-jira">` + strings.TrimSpace(data) + `</div>`
}
