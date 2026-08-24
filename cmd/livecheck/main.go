package main

import (
	"fmt"
	agent "github.com/cfpperche/agentdeck/internal/agent"
)

func main() {
	reg := agent.NewRegistry(agent.EnvWhich(nil))
	for _, a := range reg.List() {
		fmt.Printf("%s: BuildLive=%v ParseLive=%v\n", a.ID, a.BuildLive != nil, a.ParseLive != nil)
	}
}
