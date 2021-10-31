package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
)

type NodeResp struct {
	Node   *api.Node
	NodeID string
	Error  error
}

var (
	shouldHaveConsul  bool
	shouldBeIneligble bool
)

func init() {
	flag.BoolVar(&shouldHaveConsul, "consul", false, "Return nodes that have a working Consul agent.")
	flag.BoolVar(&shouldBeIneligble, "ineligible", false, "Return nodes that are ineligible.")
}

func main() {
	flag.Parse()

	c, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		fmt.Printf("failed to initialize Nomad client: %v", err)
		os.Exit(1)
	}

	nodeStubs, _, err := c.Nodes().List(nil)
	if err != nil {
		fmt.Printf("failed to initialize Nomad client: %v", err)
		os.Exit(1)
	}

	fmt.Printf("=> Found %d nodes\n", len(nodeStubs))

	nodeStatus := "Eligible"
	if shouldBeIneligble {
		nodeStatus = "Ineligible"
	}
	consulStatus := "without"
	if shouldHaveConsul {
		consulStatus = "with"
	}
	fmt.Printf("=> %s nodes %s Consul:\n\n", nodeStatus, consulStatus)

	var wg sync.WaitGroup
	wg.Add(len(nodeStubs))

	ch := make(chan NodeResp)
	for _, n := range nodeStubs {
		go func(n *api.NodeListStub) {
			defer wg.Done()
			node, _, err := c.Nodes().Info(n.ID, nil)
			ch <- NodeResp{
				Node:   node,
				NodeID: n.ID,
				Error:  err,
			}
		}(n)
	}

	go func() {
		counter := 1
		for r := range ch {
			if r.Error != nil {
				fmt.Printf("Failed to query node %s", r.NodeID)
				continue
			}

			consulVersion := r.Node.Attributes["consul.version"]
			hasConsul := consulVersion != ""
			isEligible := r.Node.SchedulingEligibility == "eligible"

			shouldPrint := shouldHaveConsul == hasConsul && shouldBeIneligble == !isEligible
			if shouldPrint {
				fmt.Printf("   %s - %s - %s\n", r.Node.Attributes["unique.network.ip-address"], r.Node.Name, r.Node.ID)
			}

			if counter%50 == 0 {
				fmt.Printf("Processed %d/%d\n", counter, len(nodeStubs))
			}
		}
	}()

	wg.Wait()
	close(ch)

	// Wait for stdout to flush.
	time.Sleep(1 * time.Second)
}
