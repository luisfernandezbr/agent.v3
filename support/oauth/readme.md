### Description

This is a very small program to help us get auth tokens from oath2 services.
You need to provide the following:

 - A **provider.yaml** file, take a look at `office365_cal.yml` or `google_cal.yml` as examples
 - An **out.json** file where the credentials will be stored
 - The `client_id` and `secret_id` of the service

### Example provider.yaml

Looks like this (google_cal.yml)

```
---
auth_uri: "https://accounts.google.com/o/oauth2/v2/auth"
token_uri: "https://www.googleapis.com/oauth2/v4/token"
scope: "https://www.googleapis.com/auth/calendar.events.readonly https://www.googleapis.com/auth/calendar.readonly"
extra_values:
  prompt: "consent"
  include_granted_scopes: "false"
  display: "page"
  access_type: "offline"
```

### Usage

Pass the following

```
go run main.go \
  --provider google_cal.yml \
  --out google_cred.json \
  --client_id "..........." \
  --client_secret "..........."
```

You can also pass in a **creds.yaml** if its easier for you

```
go run main.go \
  --provider google_cal.yml \
  --out google_auth.json \
  --creds google_creds.yml
```

### Example creds.yml

This is what the creds yaml has to look like

```
---
client_id "..........." 
client_secret "..........."
```
