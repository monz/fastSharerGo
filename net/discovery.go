package net

import (
        "net"
        "log"
        "time"
        "github.com/google/uuid"
)

const MSG_SIZE = 36

type service interface {
        Start()
        Stop()
}

type discoveryService struct {
        uuid uuid.UUID
        c *net.UDPConn
        initDelay time.Duration
        period time.Duration
}

func NewDiscoveryService(initDelay time.Duration, period time.Duration) *discoveryService {
        s := new(discoveryService)
        s.initDelay = initDelay
        s.period = period

        localAddress, err := net.ResolveUDPAddr("udp", "0.0.0.0:9942")
        if err != nil {
                log.Fatal(err)
        }
        s.c, err = net.ListenUDP("udp", localAddress)
        if err != nil {
                log.Fatal(err)
        }

        // generate unique node id (UUID)
        s.uuid = uuid.New()

        return s
}

func (s discoveryService) Start() {
        go s.receive()
        go s.send()
}

func (s discoveryService) Stop() {
        // currently will crash due to log.Fatal on closed connection!
        s.c.Close()
}

func (s discoveryService) receive() {
        log.Println("in receive", s.initDelay, s.period)
        for {
                buf := make([]byte, MSG_SIZE)
                s.c.ReadFrom(buf)
                log.Println(string(buf))
        }
}

func (s discoveryService) send() {
        log.Println("in send", s.initDelay, s.period)
        // wait initial delay
        time.Sleep(time.Second * s.initDelay)

        // define broadcast address
        destAddress, err := net.ResolveUDPAddr("udp", "255.255.255.255:9942")
        if err != nil {
                log.Fatal(err)
        }

        // convert uuid to []byte
        buf, err := s.uuid.MarshalText()
        if err != nil {
                log.Fatal(err)
        }
        // send uuid
        for {
                log.Println("send id")

                n, err := s.c.WriteToUDP(buf, destAddress)
                if err != nil {
                        log.Fatal(err)
                }
                if n != MSG_SIZE {
                        log.Fatal("Could not write discovery message properly!")
                }
                // wait
                time.Sleep(time.Second * s.period)
        }
}
