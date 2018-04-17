package main

import (
    "fmt"
    "sync"
    "net"
    "log"
    nnet "github.com/monz/fastSharerGo/net"
)

func main() {
        msgSize := 36

        fmt.Println("Receive discovery messages")
        fmt.Println("MsgSize: ", msgSize)
        // blocking call
        var wg sync.WaitGroup
        wg.Add(2)
        localAddress, err := net.ResolveUDPAddr("udp", "0.0.0.0:9942")
        if err != nil {
                log.Fatal(err)
        }

        s, err := net.ListenUDP("udp", localAddress)
        defer s.Close()
        go nnet.Receive(s, msgSize, wg)
        go nnet.Send(s, msgSize, 0, 5, wg)

        wg.Wait()
}
