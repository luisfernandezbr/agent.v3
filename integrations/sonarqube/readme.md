# Sonarqube integration

https://docs.sonarqube.org/display/SONARQUBE43/Web+Service+API

# Export command

```
Integrations JSON:
{
	"name":"sonarqube",
	"config": {
		"api_token": API_TOKEN,               // required
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
    --integrations-json='[{"name":"sonarqube", "config":{"api_token":API_TOKEN, "url":SONARQUBE_URL_ENDPOINT,"metrics":METRICS_ARRAY}}]' \
    --pinpoint-root=$HOME/.pinpoint/next-sonarqube
```
