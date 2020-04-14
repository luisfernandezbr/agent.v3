package cmdmutperf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

const perfMsg = "mutation perf stats (ms)"

func Run() {
	sc := bufio.NewScanner(os.Stdin)

	var stats map[string][]int

	for sc.Scan() {
		var log map[string]string
		err := json.Unmarshal(sc.Bytes(), &log)
		if err != nil {
			panic(err)
		}
		process := func(k string) {
			v := log[k]
			n, err := strconv.Atoi(v)
			if err != nil {
				panic(err)
			}
			stats[k] = append(stats[k], n)
		}
		if log["msg"] == perfMsg {
			if log["had_error"] != "false" {
				continue
			}
			process("webapp_to_operator")
			process("operator_to_agent")
			process("agent")
			process("agent_to_operator")
			process("total")
		}
	}

	if len(stats) == 0 {
		fmt.Println("no data found")
	}

	for k, vv := range stats {
		fmt.Println(k, "avg", avg(vv), "min", min(vv), "max", max(vv), "c", len(vv))
	}
}

func avg(arr []int) int {
	if len(arr) == 0 {
		return 0
	}

	sum := 0
	for _, v := range arr {
		sum += v
	}
	return sum / len(arr)
}

func min(arr []int) int {
	if len(arr) == 0 {
		return 0
	}
	res := arr[0]
	for _, v := range arr {
		if v < res {
			res = v
		}
	}
	return res
}

func max(arr []int) int {
	if len(arr) == 0 {
		return 0
	}
	res := arr[0]
	for _, v := range arr {
		if v > res {
			res = v
		}
	}
	return res
}
