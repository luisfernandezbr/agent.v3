package cmdforcehistorical

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	pjson "github.com/pinpt/go-common/json"
)

func Run(logger hclog.Logger, integrationName string, lastProcessed string, dedup string) error {

	var lastProcessedMap map[string]string
	if err := pjson.ReadFile(lastProcessed, &lastProcessedMap); err != nil {
		return err
	}
	newLastProcessedMap := map[string]string{}
	for key, value := range lastProcessedMap {
		if !strings.HasPrefix(key, integrationName) {
			newLastProcessedMap[key] = value
		}
	}
	data, err := json.Marshal(newLastProcessedMap)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(lastProcessed, data, 0755); err != nil {
		return err
	}

	var dedupMap map[string]interface{}
	if err := pjson.ReadFile(dedup, &dedupMap); err != nil {
		return err
	}
	delete(dedupMap, integrationName)
	data, err = json.Marshal(dedupMap)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(dedup, data, 0755); err != nil {
		return err
	}
	return nil
}
