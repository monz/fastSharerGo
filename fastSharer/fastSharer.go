package main

import (
	"flag"
	"fmt"
	"github.com/monz/fastSharerGo/local/services"
	nnet "github.com/monz/fastSharerGo/net/services"
	"sync"
	"time"
)

var shareInfoPeriod time.Duration
var downloadDir string
var shareDir string
var shareDirRecursive bool
var cmdPort int
var discoveryPort int
var discoveryPeriod time.Duration
var maxDownloads int
var maxUploads int

func init() {
	flag.DurationVar(&shareInfoPeriod, "shareInfoPeriod", 5*time.Second, "number of seconds shared file info messages get send")
	flag.StringVar(&downloadDir, "downloadDir", "./", "download directory")
	flag.StringVar(&shareDir, "shareDir", "./share", "shared files directory")
	flag.BoolVar(&shareDirRecursive, "recursive", true, "recursively watch shared files directory")
	flag.IntVar(&cmdPort, "cmdPort", 6132, "port for share command messages")
	flag.IntVar(&discoveryPort, "discoPort", 9942, "port for node discovery")
	flag.DurationVar(&discoveryPeriod, "discoPeriod", 5*time.Second, "number of seconds discovery messages get send")
	flag.IntVar(&maxDownloads, "maxDown", 5, "max concurrent download connections")
	flag.IntVar(&maxUploads, "maxUp", 5, "max concurrent upload connections")
}

// todo: load all settings from config file or cmd line
func main() {
	flag.Parse()
	fmt.Println("Starting fastSharer...")

	// start services
	discoService := nnet.NewDiscoveryService(discoveryPort, 0, discoveryPeriod)
	discoService.Start()

	netService := nnet.NewNetworkService(discoService.LocalNodeId(), cmdPort)
	netService.Start()
	// subscribe to node message updates from discovery service
	discoService.Register(netService)

	shareService := nnet.NewShareService(netService.LocalNodeId(), netService.Sender(), shareInfoPeriod, downloadDir, maxUploads, maxDownloads)
	shareService.Start()
	// subscribe to share message updates from network service
	netService.Register(shareService)
	// subscribe to node message updates from dicovery service
	discoService.Register(shareService)

	fileDiscoService := local.NewFileDiscoveryService(shareDir, shareDirRecursive)
	fileDiscoService.Start()
	// subscribe to file change updates from file discovery service
	fileDiscoService.Register(shareService)

	// future: open 'shell' to handle sharer control commands from command line
	// intermediate solution, blocking call
	var wg sync.WaitGroup
	wg.Add(1)

	wg.Wait()
}
