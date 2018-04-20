package net

import (
        "net"
        "log"
        "time"
        "github.com/monz/fastSharerGo/net/data"
        "github.com/google/uuid"
)

const MSG_SIZE = 36

type discoveryService struct {
        uuid uuid.UUID
        c *net.UDPConn
        initDelay time.Duration
        period time.Duration
        node chan *data.Node
        localIps map[string]bool
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
        s.localIps = extractLocalAdresses()

        return s
}

func extractLocalAdresses() map[string]bool {
        ifaces, err := net.Interfaces()
        if err != nil {
                log.Fatal(err)
        }
        ipAdresses := make(map[string]bool)
        for _, iface := range ifaces {
                if iface.Flags & net.FlagUp == 0 {
                        continue // interface down
                }
                if iface.Flags & net.FlagLoopback != 0 {
                        continue // loopback interface
                }
                addrs, err := iface.Addrs()
                if err != nil {
                        log.Fatal(err)
                }
                for _, addr := range addrs {
                        var ip net.IP
                        switch v := addr.(type) {
                        case *net.IPNet:
                                ip = v.IP
                        case *net.IPAddr:
                                ip = v.IP
                        }
                        if ip == nil || ip.IsLoopback() {
                                continue
                        }
                        ip = ip.To4()
                        if ip == nil {
                                continue // not an ipv4 address
                        }
                        ipAdresses[ip.String()] = true
                }
        }
        return ipAdresses
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

                addr := s.c.RemoteAddr()
                if addr == nil {
                        continue
                }
                ip := addr.(*net.IPAddr)
                remoteIp4 := ip.IP.To4()
                if remoteIp4 == nil {
                        continue // not an ipv4 address
                }
                remoteIp := remoteIp4.String()

                _, isLocal := s.localIps[remoteIp]
                if !isLocal {
                        id, err := uuid.ParseBytes(buf)
                        if err != nil {
                                log.Println(err)
                                continue
                        }
                        node := data.NewNode(id, remoteIp)
                        s.node <- node
                }
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
