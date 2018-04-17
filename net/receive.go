package net

import (
        "net"
        "log"
        "sync"
        "time"
        "github.com/google/uuid"
)

func Receive(s *net.UDPConn, msgSize int, wg sync.WaitGroup) {
        defer wg.Done()
        for {
                buf := make([]byte, msgSize)
                s.ReadFrom(buf)
                log.Println(string(buf))
        }
}

func Send(s *net.UDPConn, msgSize int, initialDelay time.Duration, period time.Duration, wg sync.WaitGroup) {
        defer wg.Done()
        // wait initial delay
        time.Sleep(time.Second * initialDelay)

        // define broadcast address
        destAddress, err := net.ResolveUDPAddr("udp", "255.255.255.255:9942")
        if err != nil {
                log.Fatal(err)
        }

        // generate unique node id (UUID)
        uuid := uuid.New()
        buf, err := uuid.MarshalText()
        if err != nil {
                log.Fatal(err)
        }

        for {
                // send uuid
                log.Println("send id")

                n, err := s.WriteToUDP(buf, destAddress)
                if err != nil {
                        log.Fatal(err)
                }
                if n != msgSize {
                        log.Fatal("Could not write discovery message properly!")
                }
                // wait
                time.Sleep(time.Second * period)
        }
}
