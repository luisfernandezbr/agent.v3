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

func TestAdjustRenderedHTML(t *testing.T) {
	cases := []struct {
		Label      string
		In         string
		WebsiteURL string
		Want       string
	}{
		{
			Label:      "empty",
			In:         "",
			WebsiteURL: "",
			Want:       "",
		},
		{
			Label:      "basic string",
			In:         "<p>example</p>",
			WebsiteURL: "",
			Want:       `<div class="source-jira"><p>example</p></div>`,
		},
		{
			Label:      "fixing image links",
			In:         "<p>something something</p>\n\n<p> <span class=\"image-wrap\" style=\"\"><a id=\"11887_thumb\" href=\"/secure/attachment/11887/11887_suggested+work+mentors.png\" title=\"suggested work mentors.png\" file-preview-type=\"image\" file-preview-id=\"11887\" file-preview-title=\"suggested work mentors.png\"><jira-attachment-thumbnail url=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png?default=false\" jira-url=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png\" filename=\"suggested work mentors.png\"><img src=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png\" data-attachment-name=\"suggested work mentors.png\" data-attachment-type=\"thumbnail\" data-media-services-id=\"0f465f89-3c54-46d7-a311-cce52b438b85\" data-media-services-type=\"file\" style=\"border: 0px solid black\" /></jira-attachment-thumbnail></a></span> </p>",
			WebsiteURL: "",
			Want: `<div class="source-jira"><p>something something</p>

<p> <span class="image-wrap" style=""><img src="https://example.com/secure/thumbnail/11887/suggested+work+mentors.png" /></span> </p></div>`,
		},
		{
			Label:      "fixing emoticons",
			In:         "<p><img class=\"emoticon\" src=\"/images/icons/emoticons/smile.png\" height=\"16\" width=\"16\" align=\"absmiddle\" alt=\"\" border=\"0\"/> xzs</p>",
			WebsiteURL: "https://example.com",
			Want:       `<div class="source-jira"><p><img class="emoticon" src="https://example.com/images/icons/emoticons/smile.png" height="16" width="16" align="absmiddle" alt="" border="0"/> xzs</p></div>`,
		},
	}
	for _, c := range cases {
		got := adjustRenderedHTML(c.WebsiteURL, c.In)
		if got != c.Want {
			t.Errorf("failed case\n%v\nwant\n%v\ngot\n%v", c.Label, c.Want, got)
		}
	}
}
