package main

import "github.com/pinpt/agent.next/integrations/pkg/ibase"

func main() {
	ibase.MainFunc(NewIntegration)
}
