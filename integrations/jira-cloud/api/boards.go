package api

import (
	"net/url"
	"strconv"

	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/work"
)

func BoardsPage(
	qc QueryContext,
	paginationParams url.Values) (pi PageInfo, res []*work.KanbanBoard, _ error) {

	objectPath := "board"
	params := paginationParams

	qc.Logger.Debug("boards request", "params", params)

	var rr struct {
		Total      int  `json:"total"`
		MaxResults int  `json:"maxResults"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"values"`
	}

	err := qc.Req.GetAgile(objectPath, params, &rr)
	if err != nil {
		return pi, res, err
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Values) != 0 {
		pi.HasMore = !rr.IsLast
	}

	for _, data := range rr.Values {
		item := &work.KanbanBoard{}
		item.CustomerID = qc.CustomerID
		item.RefID = strconv.FormatInt(data.ID, 10)
		item.RefType = "jira"
		item.Name = data.Name
		res = append(res, item)
	}

	return pi, res, nil
}

// BoardColumnsStatuses get board columns and statuses
func BoardColumnsStatuses(
	qc QueryContext,
	issueStatuses map[string]*work.IssueStatus,
	boardID string,
) (res []work.KanbanBoardColumns, _ error) {

	objectPath := pstrings.JoinURL("board", boardID, "configuration")

	qc.Logger.Debug("board configuration request", "board_id", boardID)

	var cc struct {
		ColumnConfig struct {
			Columns []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID string `json:"id"`
				} `json:"statuses"`
			} `json:"columns"`
		} `json:"columnConfig"`
	}

	err := qc.Req.GetAgile(objectPath, nil, &cc)
	if err != nil {
		return res, err
	}

	for _, column := range cc.ColumnConfig.Columns {

		statusIds := make([]string, 0)
		for _, status := range column.Statuses {

			issueStatus, ok := issueStatuses[status.ID]
			if !ok {
				qc.Logger.Warn("status does not exist or board is empty", "board", boardID)
				continue
			}

			statusIds = append(statusIds, issueStatus.ID)
		}

		res = append(res, work.KanbanBoardColumns{
			Name:      column.Name,
			StatusIds: statusIds,
		})
	}

	return res, nil
}

// BoardProjectListPage get the list of projects for this board
func BoardProjectListPage(
	boardID string,
	qc QueryContext,
	paginationParams url.Values) (pi PageInfo, res []string, _ error) {

	params := paginationParams

	objectPath := pstrings.JoinURL("board", boardID, "project", "full")

	qc.Logger.Debug("board project list request")

	var cc struct {
		Total      int  `json:"total"`
		MaxResults int  `json:"maxResults"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			ID string `json:"id"`
		} `json:"values"`
	}

	err := qc.Req.GetAgile(objectPath, params, &cc)
	if err != nil {
		return pi, res, err
	}

	pi.Total = cc.Total
	pi.MaxResults = cc.MaxResults
	if len(cc.Values) != 0 {
		pi.HasMore = !cc.IsLast
	}

	for _, project := range cc.Values {
		res = append(res, project.ID)
	}

	return pi, res, nil
}
