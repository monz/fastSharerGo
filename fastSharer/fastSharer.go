package main

import (
	"fmt"
	"github.com/monz/fastSharerGo/data"
	nnet "github.com/monz/fastSharerGo/net/services"
	"log"
	"sync"
)

func main() {
	fmt.Println("Starting fastSharer...")

	// start services
	discoService := nnet.NewDiscoveryService(0, 5)
	discoService.Start()

	netService := nnet.NewNetworkService(6132)
	netService.Start()

	//shareService := nnet.ShareService()
	//shareService.Start()
	// subscribe to share message updates from network service
	//netService.Subscribe(shareService)

	file := data.NewFileMetadata("/home/markus/tmp/testo.txt", "")
	log.Println(file)

	// future: open 'shell' to handle sharer control commands from command line
	// intermediate solution, blocking call
	var wg sync.WaitGroup
	wg.Add(1)

	wg.Wait()
}
