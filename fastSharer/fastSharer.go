package main

import (
	"flag"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/gui"
	"github.com/monz/fastSharerGo/local/services"
	nnet "github.com/monz/fastSharerGo/net/services"
	"log"
	//"sync"
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
var checksumWorker int

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
	flag.IntVar(&checksumWorker, "sumWorker", 1, "max concurrent checksum worker")
}

// todo: load all settings from config file or cmd line
func main() {
	flag.Parse()
	log.Println("Starting fastSharer...")

	// prepare user interface
	gui := ui.NewShareUi()
	if err := gui.Init(); err != nil {
		log.Fatal(err)
	}

	// start services
	// node discovery service
	discoService := nnet.NewDiscoveryService(discoveryPort, 0, discoveryPeriod)
	discoService.Start()

	// network service
	netService := nnet.NewNetworkService(discoService.LocalNodeId(), cmdPort)
	netService.Start()
	// subscribe to node message updates from discovery service
	discoService.Register(netService)
	discoService.Register(gui)

	// prepare dependencies for share service
	sender := nnet.NewShareSender(netService.Sender())
	uploader := nnet.NewShareUploader(netService.LocalNodeId(), maxUploads, sender)
	downloader := nnet.NewShareDownloader(netService.LocalNodeId(), maxDownloads, sender)
	// share service
	shareService := nnet.NewShareService(netService.LocalNodeId(), sender, uploader, downloader, shareInfoPeriod, downloadDir)
	shareService.Start()

	// subscribe to share message updates from network service
	netService.Register(gui)
	netService.Register(shareService)
	// subscribe to node message updates from dicovery service
	discoService.Register(shareService)

	// file discovery service
	checksumService := util.NewShareChecksumService(checksumWorker)
	fileDiscoService := local.NewFileDiscoveryService(checksumService, shareDir, shareDirRecursive)

	// subscribe to file change updates from file discovery service
	// register subscriber before start, because they get directly informed when services starts
	fileDiscoService.Register(shareService)
	fileDiscoService.Register(gui)
	fileDiscoService.Start()

	// show user interface
	gui.Show()
}
