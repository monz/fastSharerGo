package data

import (
        "container/list"
        "net"
        "github.com/google/uuid"
)

type Node struct {
        id uuid.UUID
        ips map[string]bool
        lastTimeSeen int64
        socket net.TCPConn
}

func NewNode(id uuid.UUID, ip string) *Node {
        node := new(Node)
        node.id = id
        node.ips = make(map[string]bool)
        node.ips[ip] = true

        return node
}

func (n Node) Id() uuid.UUID {
        return n.id
}

func (n Node) Ips() *list.List {
        ips := list.New()
        for k, _ := range n.ips {
                ips.PushBack(k)
        }
        return ips
}

func (n Node) LastTimeSeen() int64 {
        return n.lastTimeSeen
}

func (n Node) SetLastTimeSeen(lastTimeSeen int64) {
        n.lastTimeSeen = lastTimeSeen
}

func (n Node) Connect(ip string, port int, timeout int) {
        // todo: Implement
}
