package cmdexport

/*
// SendProgressStatus sends an agent export progress status event
func SendProgressStatus(ctx context.Context, logger log.Logger, channel string, apiKey string, mainProgress *progress.MainProgress) error {
	data := mainProgress.GetJSONStr()
	agentEvent := &agent.ExportResponse{
		JobID:   mainProgress.JobID,
		RefType: "progress",
		Type:    agent.ExportResponseTypeExport,
		Data:    &data,
		Success: true,
	}
	return send(ctx, logger, agentEvent, channel, apiKey, mainProgress.JobID, map[string]string{"action": "progress"})
}
*/
