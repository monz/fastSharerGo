package main

import (
    "fmt"
    "sync"
    nnet "github.com/monz/fastSharerGo/net/services"
    "github.com/monz/fastSharerGo/data"
    "log"
)


func main() {
        fmt.Println("Starting fastSharer...")

        // start services
        discoService := nnet.NewDiscoveryService(0, 5)
        discoService.Start()

        file := data.NewFileMetadata("/home/markus/tmp/testo.txt", "")
        log.Println(file)

        // future: open 'shell' to handle sharer control commands from command line
        // intermediate solution, blocking call
        var wg sync.WaitGroup
        wg.Add(1)

        wg.Wait()
}
