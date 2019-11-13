package build

import (
	"os"
	"strings"
)

func BuiltinIntegrationBinaries() []string {
	all := os.Getenv("PP_INTEGRATION_BINARIES_ALL")
	if all == "" {
		return nil
	}
	return strings.Split(all, ",")
}
