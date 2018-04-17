package main

import (
    "fmt"
    "sync"
    nnet "github.com/monz/fastSharerGo/net"
    "github.com/monz/fastSharerGo/data"
    "log"
)


func main() {
        fmt.Println("Receive discovery messages")
        // blocking call
        var wg sync.WaitGroup
        wg.Add(2)

        discoService := nnet.NewDiscoveryService(0, 5)
        discoService.Start()

        file := data.NewFileMetadata("/home/markus/tmp/testo.txt", "")
        log.Println(file)

        wg.Wait()
}
