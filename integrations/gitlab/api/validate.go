package api

func ValidateUser(qc QueryContext) (err error) {
	qc.Logger.Debug("user request")

	_, err = qc.Request("user", nil, nil)

	return
}
