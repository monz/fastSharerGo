package net

import (
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"net"
	"time"
)

const MSG_SIZE = 36

type DiscoveryService struct {
	uuid       uuid.UUID
	c          *net.UDPConn
	initDelay  time.Duration
	period     time.Duration
	localIps   map[string]bool
	subscriber []data.NodeSubscriber
}

func NewDiscoveryService(initDelay time.Duration, period time.Duration) *DiscoveryService {
	d := new(DiscoveryService)
	d.initDelay = initDelay
	d.period = period

	localAddress, err := net.ResolveUDPAddr("udp", "0.0.0.0:9942")
	if err != nil {
		log.Fatal(err)
	}
	d.c, err = net.ListenUDP("udp", localAddress)
	if err != nil {
		log.Fatal(err)
	}

	// generate unique node id (UUID)
	d.uuid = uuid.New()
	d.localIps = extractLocalAdresses()

	return d
}

func (d *DiscoveryService) Register(subscriber data.NodeSubscriber) {
	d.subscriber = append(d.subscriber, subscriber)
}

func extractLocalAdresses() map[string]bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}
	ipAdresses := make(map[string]bool)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
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

func (d *DiscoveryService) Start() {
	go d.receive()
	go d.send()
}

func (d DiscoveryService) Stop() {
	// currently will crash due to log.Fatal on closed connection!
	d.c.Close()
}

func (d *DiscoveryService) receive() {
	log.Println("in receive", d.initDelay, d.period)
	for {
		buf := make([]byte, MSG_SIZE)
		n, addr, err := d.c.ReadFrom(buf)
		if n != MSG_SIZE {
			log.Println("Malformed message received!")
		} else if err != nil {
			log.Println(err)
		} else if addr == nil {
			log.Println("Could not determine remote address")
			continue
		}

		log.Println(string(buf))

		ip := addr.(*net.UDPAddr)
		remoteIp4 := ip.IP.To4()
		if remoteIp4 == nil {
			continue // not an ipv4 address
		}
		remoteIp := remoteIp4.String()

		_, isLocal := d.localIps[remoteIp]
		if !isLocal {
			id, err := uuid.ParseBytes(buf)
			if err != nil {
				log.Println(err)
				continue
			}
			node := data.NewNode(id, remoteIp)
			d.updateAddNode(*node)
		}
	}
}

func (d *DiscoveryService) updateAddNode(node data.Node) {
	for _, s := range d.subscriber {
		s.AddNode(node)
	}
}

func (d DiscoveryService) send() {
	log.Println("in send", d.initDelay, d.period)
	// wait initial delay
	time.Sleep(time.Second * d.initDelay)

	// define broadcast address
	destAddress, err := net.ResolveUDPAddr("udp", "255.255.255.255:9942")
	if err != nil {
		log.Fatal(err)
	}

	// convert uuid to []byte
	buf, err := d.uuid.MarshalText()
	if err != nil {
		log.Fatal(err)
	}
	// send uuid
	for {
		log.Println("send id")

		n, err := d.c.WriteToUDP(buf, destAddress)
		if err != nil {
			log.Fatal(err)
		}
		if n != MSG_SIZE {
			log.Fatal("Could not write discovery message properly!")
		}
		// wait
		time.Sleep(time.Second * d.period)
	}
}
