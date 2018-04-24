package data

import (
	"fmt"
	"github.com/google/uuid"
	"net"
	"time"
)

type Node struct {
	id           uuid.UUID
	ips          []string
	lastTimeSeen int64
	conn         *net.TCPConn
}

func NewNode(id uuid.UUID, ip string) *Node {
	node := new(Node)
	node.id = id
	node.ips = append(node.ips, ip)

	return node
}

func (n Node) Id() uuid.UUID {
	return n.id
}

func (n Node) Ips() []string {
	return n.ips
}

func (n Node) LastTimeSeen() int64 {
	return n.lastTimeSeen
}

func (n Node) SetLastTimeSeen(lastTimeSeen int64) {
	n.lastTimeSeen = lastTimeSeen
}

func (n *Node) Connect(ip string, port int, timeout time.Duration) (*net.TCPConn, error) {
	if n.conn == nil {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
		if err != nil {
			return nil, err
		}
		tcpConn, ok := conn.(*net.TCPConn)
		if !ok {
			return nil, err
		}
		n.conn = tcpConn
	}

	return n.conn, nil
}
