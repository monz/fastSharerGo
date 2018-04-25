package main

import (
	"fmt"
	nnet "github.com/monz/fastSharerGo/net/services"
	"sync"
)

// todo: load all settings from config file or cmd line
func main() {
	fmt.Println("Starting fastSharer...")

	// start services
	discoService := nnet.NewDiscoveryService(0, 5)
	discoService.Start()

	netService := nnet.NewNetworkService(discoService.LocalNodeId(), 6132)
	netService.Start()
	// subscribe to node message updates from discovery service
	discoService.Register(netService)

	shareService := nnet.NewShareService(netService.LocalNodeId(), netService.Sender(), "/var/vms/dl_tmp10", 5, 5)
	shareService.Start()
	// subscribe to share message updates from network service
	netService.Register(shareService)
	// subscribe to node message updates from dicovery service
	discoService.Register(shareService)

	// future: open 'shell' to handle sharer control commands from command line
	// intermediate solution, blocking call
	var wg sync.WaitGroup
	wg.Add(1)

	wg.Wait()
}
