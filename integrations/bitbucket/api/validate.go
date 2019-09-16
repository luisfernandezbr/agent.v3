package api

func ValidateUser(qc QueryContext) (rerr error) {
	qc.Logger.Debug("user request")

	_, err := qc.Request("user", nil, false, nil)
	if err != nil {
		rerr = err
	}

	return
}
