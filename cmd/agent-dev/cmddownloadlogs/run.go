package cmddownloadlogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	pstrings "github.com/pinpt/go-common/strings"
)

type Opts struct {
	URL      string
	User     string
	Password string

	AgentUUID  string
	CustomerID string

	NoFormat bool
}

type jm map[string]interface{}

func Run(opts Opts) {
	if opts.URL == "" || opts.User == "" || opts.Password == "" || opts.AgentUUID == "" || opts.CustomerID == "" {
		panic("provide all required params")
	}

	//authenticate(opts)
	getData(opts)
}

/*
func authenticate(opts Opts) {

	u := pstrings.JoinURL(opts.URL, "/_security/_authenticate")
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth(opts.User, opts.Password)
	req.Header.Set("Content-Type", "application/json")

	req.SetBasicAuth(opts.User, opts.Password)
	fmt.Println("opts.P", opts.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(b))
}*/

func getData(opts Opts) {

	index := "agent-" + getIndexDate() + ".*"

	page := 0
	pageSize := 10000

	request := jm{
		"from": pageSize * page,
		"size": pageSize,
		"query": jm{
			"bool": jm{
				"must": []jm{
					jm{"match": jm{"fields.uuid": opts.AgentUUID}},
					jm{"match": jm{"fields.customer_id": opts.CustomerID}},
				},
			},
		},
		"sort": []jm{
			{"fields.@timestamp": jm{"order": "desc"}},
		},
	}

	b, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}

	u := pstrings.JoinURL(opts.URL, index, "_search")

	req, err := http.NewRequest("GET", u, bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth(opts.User, opts.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	if opts.NoFormat {
		fmt.Println(string(b))
		return
	}

	var res struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Fields map[string]interface{} `json:"fields"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	err = json.Unmarshal(b, &res)
	if err != nil {
		panic(err)
	}

	for _, hit := range res.Hits.Hits {
		fields := hit.Source.Fields
		lvl := fields["@level"]
		msg := fields["@message"]
		ts := fields["@timestamp"]
		fmt.Print(lvl, " ", ts, " ", msg, " ")
		delete(fields, "@level")
		delete(fields, "@message")
		delete(fields, "@timestamp")

		delete(fields, "message_id")
		delete(fields, "customer_id")
		delete(fields, "uuid")
		b, err := json.Marshal(fields)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
	}

}

func getIndexDate() string {
	return time.Now().UTC().Format("2006.01")
}
