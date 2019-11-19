package build

import (
	"errors"
	"os"
	"strings"

	"github.com/blang/semver"
)

func BuiltinIntegrationBinaries() []string {
	all := os.Getenv("PP_INTEGRATION_BINARIES_ALL")
	if all == "" {
		return nil
	}
	return strings.Split(all, ",")
}

func ValidateVersion(v string) error {
	if v == "" {
		return errors.New("version required")
	}
	if v == "test" {
		return nil
	}
	if !strings.HasPrefix(v, "v") {
		return errors.New("version must start with v")
	}
	v = v[1:]
	_, err := semver.New(v)
	if err != nil {
		return err
	}
	return nil
}
