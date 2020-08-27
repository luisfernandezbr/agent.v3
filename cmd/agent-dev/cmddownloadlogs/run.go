package cmddownloadlogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	pstrings "github.com/pinpt/go-common/v10/strings"
)

type Opts struct {
	URL      string
	User     string
	Password string

	AgentUUID  string
	CustomerID string

	Format string

	MaxRecords int
}

type jm map[string]interface{}

func Run(opts Opts) {
	if opts.URL == "" || opts.User == "" || opts.Password == "" || opts.CustomerID == "" {
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
	maxRecords := opts.MaxRecords
	if maxRecords == 0 {
		maxRecords = 10000
	}
	maxPageSize := 10000

	index := "agent-" + getIndexDate() + ".*"

	searchAfter := 0

	total := 0

	for {
		pageSize := maxPageSize
		toGo := maxRecords - total
		if toGo < maxPageSize {
			pageSize = toGo
		}

		matches := []jm{
			jm{"match": jm{"fields.customer_id": opts.CustomerID}},
		}

		if opts.AgentUUID != "" {
			matches = append(matches, jm{"match": jm{"fields.uuid": opts.AgentUUID}})
		}

		request := jm{
			"size": pageSize,
			"query": jm{
				"bool": jm{
					"must": matches,
				},
			},
			"sort": []jm{
				{"fields.timestamp": jm{"order": "desc"}},
			},
		}

		if searchAfter != 0 {
			request["search_after"] = []int{searchAfter}
		}

		b, err := json.Marshal(request)
		if err != nil {
			panic(err)
		}

		u := pstrings.JoinURL(opts.URL, index, "_search")

		req, err := http.NewRequest("POST", u, bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		req.SetBasicAuth(opts.User, opts.Password)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("kbn-xsrf", "true")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		if opts.Format == "source" {
			fmt.Println(string(b))
			return
		}

		var res struct {
			Hits struct {
				Hits []struct {
					Source struct {
						Severity string                 `json:"severity"`
						Message  string                 `json:"message"`
						Fields   map[string]interface{} `json:"fields"`
					} `json:"_source"`
					Sort []int `json:"sort"`
				} `json:"hits"`
			} `json:"hits"`
		}

		err = json.Unmarshal(b, &res)
		if err != nil {
			panic(err)
		}

		for _, hit := range res.Hits.Hits {
			if len(hit.Sort) == 0 {
				panic("no sort provided")
			} else {
				searchAfter = hit.Sort[0]
			}

			if opts.Format == "json" {
				b, err := json.Marshal(hit.Source)
				if err != nil {
					panic(err)
				}
				fmt.Println(string(b))
				continue
			}

			fields := hit.Source.Fields
			lvl := hit.Source.Severity
			msg := hit.Source.Message
			ts := fields["timestamp"]
			fmt.Print(lvl, " ", ts, " ", msg, " ")
			delete(fields, "timestamp")

			delete(fields, "message_id")
			delete(fields, "customer_id")
			delete(fields, "uuid")
			b, err := json.Marshal(fields)
			if err != nil {
				panic(err)
			}

			fmt.Println(string(b))
		}

		if len(res.Hits.Hits) == 0 {
			return
		}

		total += len(res.Hits.Hits)

		if total >= maxRecords {
			return
		}
	}

}

func getIndexDate() string {
	return time.Now().UTC().Format("2006.01")
}
