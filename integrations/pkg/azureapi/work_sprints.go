package azureapi

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

type sprintsResponse struct {
	Attributes struct {
		FinishDate time.Time `json:"finishDate"`
		StartDate  time.Time `json:"startDate"`
		TimeFrame  string    `json:"timeFrame"` // past, current, future
	} `json:"attributes"`
	ID   string `json"id"`
	Name string `json"name"`
	Path string `json"path"`
	URL  string `json"url"`
}

func (api *API) FetchSprints(projid string, sprints chan<- datamodel.Model) error {
	teams, err := api.fetchTeams(projid)
	if err != nil {
		panic(err)
	}
	a := NewAsync(5)
	for _, team := range teams {
		a.Send(AsyncMessage{
			Data: team.ID,
			Func: func(data interface{}) {
				teamid := data.(string)
				if err := api.fetchSprint(projid, teamid, sprints); err != nil {
					api.logger.Error("error fetching sprints for project "+projid+" and team "+teamid, "err", err)
				}
			},
		})
	}
	a.Wait()
	return err
}

func (api *API) fetchSprint(projid string, teamid string, sprints chan<- datamodel.Model) error {
	url := fmt.Sprintf(`%s/%s/_apis/work/teamsettings/iterations`, projid, teamid)
	var res []sprintsResponse
	err := api.getRequest(url, nil, &res)
	if err != nil {
		return err
	}
	for _, r := range res {
		sprint := work.Sprint{
			CustomerID: api.customerid,
			// Goal
			Name:    r.Name,
			RefID:   r.ID,
			RefType: api.reftype,
			// Status
		}
		date.ConvertToModel(r.Attributes.StartDate, &sprint.StartedDate)
		date.ConvertToModel(r.Attributes.FinishDate, &sprint.EndedDate)
		date.ConvertToModel(r.Attributes.FinishDate, &sprint.CompletedDate)
		date.ConvertToModel(time.Now(), &sprint.FetchedDate)
		sprints <- &sprint
	}
	return nil
}
