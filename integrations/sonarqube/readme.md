## Sonarqube integration

https://docs.sonarqube.org/display/SONARQUBE43/Web+Service+API

## Export command

```
Integrations JSON:
{
	"name":"sonarqube",
	"config": {
		"apitoken": API_TOKEN,               // required
		"url":       SONARQUBE_URL_ENDPOINT,  // required
		"metrics": [                          // optional
				"complexity","code_smells",
				"new_code_smells","sqale_rating",
				"reliability_rating","security_rating",
				"coverage","new_coverage",
				"test_success_density","new_technical_debt"
		]
	}
}
----------
go run . export \
    --agent-config-json='{"customer_id":"customer_id"}' \
    --integrations-json='[{"name":"sonarqube", "config":{"apitoken":API_TOKEN, "url":SONARQUBE_URL_ENDPOINT,"metrics":METRICS_ARRAY}}]' \
    --pinpoint-root=$HOME/.pinpoint/next-sonarqube
```

## Running tests

To run the tests you'll need to enable it with the _PP_TEST_SONARQUBE_ flag set to "1", you'll also need the api key and the api url

```
PP_TEST_SONARQUBE_URL=https://api-url PP_TEST_SONARQUBE_APIKEY=1234567890 PP_TEST_SONARQUBE=1 go test github.com/pinpt/agent.next/integrations/sonarqube...
```
