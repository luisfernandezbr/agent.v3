package api

func ValidateUser(qc QueryContext) (rerr error) {
	qc.Logger.Debug("user request")

	err := qc.Request("user", nil, nil)
	if err != nil {
		rerr = err
	}

	return
}
