package main

import (
	"fmt"
	nnet "github.com/monz/fastSharerGo/net/services"
	"sync"
)

func main() {
	fmt.Println("Starting fastSharer...")

	// start services
	discoService := nnet.NewDiscoveryService(0, 5)
	discoService.Start()

	netService := nnet.NewNetworkService(6132)
	netService.Start()
	// subscribe to node message updates from discovery service
	discoService.Subscribe(netService)

	shareService := nnet.NewShareService()
	shareService.Start()
	// subscribe to share message updates from network service
	netService.Subscribe(shareService)

	// future: open 'shell' to handle sharer control commands from command line
	// intermediate solution, blocking call
	var wg sync.WaitGroup
	wg.Add(1)

	wg.Wait()
}
