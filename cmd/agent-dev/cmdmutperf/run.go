package cmdmutperf

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const perfMsg = "mutation perf stats (ms)"

func Run() {
	sc := bufio.NewScanner(os.Stdin)

	stats := map[string][]int{}

	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, perfMsg) {
			continue
		}
		fields := strings.Split(line, " ")
		log := map[string]string{}
		for _, f := range fields {
			if !strings.Contains(f, "=") {
				continue
			}
			parts := strings.Split(f, "=")
			if len(parts) == 2 {
				log[parts[0]] = parts[1]
			}
		}
		//var log map[string]string
		//err := json.Unmarshal(sc.Bytes(), &log)
		//if err != nil {
		//panic(err)
		//}
		process := func(k string) {
			v := log[k]
			n, err := strconv.Atoi(v)
			if err != nil {
				panic(err)
			}
			stats[k] = append(stats[k], n)
		}
		if log["had_error"] != "false" {
			continue
		}
		process("webapp_to_operator")
		process("operator_to_agent")
		process("agent")
		process("agent_to_operator")
		process("total")
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
