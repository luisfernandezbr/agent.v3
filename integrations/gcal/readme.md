## Google Calendar integration

### Contents

- [Google Developer Console](https://console.developers.google.com)
- [Google Calendar API](https://developers.google.com/calendar/v3/reference)

### Getting Started

Go to `./support/oauth/` and generate a new access_token. For this, you will need to [create a new google app](https://console.developers.google.com/) and add the required scopes: `https://www.googleapis.com/auth/calendar.events.readonly`, and `https://www.googleapis.com/auth/calendar.readonly`

Then run this command:

```
dep ensure
go run main.go \
	--provider google_cal.yml \
	--out google_cred.json \
	--client_id "....." --client_secret "...."
```

It will ask you to login and then you will get an access token. Go to the next step.

### Export command

Create an `export.json` file in the root of the agent repo with the following

```
[{
    "name": "gcal",
    "type": 3,
    "config": {
        "access_token": ".....",
       	"local": true
    }
}]
```
Then run:
```
go run main.go export \
	--agent-config-json='{"customer_id":"c1"}' \
	--integrations-file=google_cal.json \
	--pinpoint-root ./tmp
```

If you get a "token expired" error, run the command in _Getting Started_ to create a new access token and update the `export.json`

### Integration

#### Export

This is a very simple integration. It will export calendars and the events within those calendars.

If you pass in an `inclusions` list (array in the `config` object of the export.json), it will try to get those calendars and its events. 

If you pass in an `exclusions` list (array in the `config` object of the export.json), or no list at all, it will fetch all the calendars you are subscribed to, but exclude those in that array, if any.


### Incremental

Google APIs use a `syncToken`, this is implemented only in the events api and not the calendar api. 

