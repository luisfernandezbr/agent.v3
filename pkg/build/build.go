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
	// test is the local production builds version
	// dev is the version that can be upload to s3 and updated to from admin
	if v == "test" || v == "dev" {
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
