package azureapi

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

// FetchSprints gets sprints from a single project and multiple teams async, and sends them to the sprint channel
func (api *API) FetchSprints(projid string, sprints chan<- datamodel.Model) error {
	teams, err := api.fetchTeams(projid)
	if err != nil {
		return err
	}
	a := NewAsync(api.concurrency)
	for _, team := range teams {
		teamid := team.ID
		a.Send(func() {
			if _, err := api.fetchSprint(projid, teamid, sprints); err != nil {
				api.logger.Error("error fetching sprints for project "+projid+" and team "+teamid, "err", err)
				return
			}
		})
	}
	a.Wait()
	return err
}

func (api *API) fetchSprint(projid string, teamid string, sprints chan<- datamodel.Model) ([]sprintsResponse, error) {
	url := fmt.Sprintf(`%s/%s/_apis/work/teamsettings/iterations`, projid, teamid)
	var res []sprintsResponse
	err := api.getRequest(url, nil, &res)
	for _, r := range res {
		sprint := work.Sprint{
			CustomerID: api.customerid,
			// Goal is missing
			Name:    r.Name,
			RefID:   r.ID,
			RefType: api.reftype,
		}
		switch r.Attributes.TimeFrame {
		case "past":
			sprint.Status = work.SprintStatusClosed
		case "current":
			sprint.Status = work.SprintStatusActive
		case "future":
			sprint.Status = work.SprintStatusFuture
		default:
			sprint.Status = work.SprintStatus(4) // unset
		}
		date.ConvertToModel(r.Attributes.StartDate, &sprint.StartedDate)
		date.ConvertToModel(r.Attributes.FinishDate, &sprint.EndedDate)
		date.ConvertToModel(r.Attributes.FinishDate, &sprint.CompletedDate)
		date.ConvertToModel(time.Now(), &sprint.FetchedDate)
		sprints <- &sprint
	}
	return res, err
}
