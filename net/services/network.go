package net

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"net"
	"sync"
	"time"
)

const (
	socketTiemout = time.Second * 5
)

type NetworkService struct {
	nodes       map[uuid.UUID]*data.Node
	cmdPort     int
	localNodeId uuid.UUID
	subscriber  []data.ShareSubscriber
	mu          sync.Mutex
}

func NewNetworkService(cmdPort int) *NetworkService {
	n := new(NetworkService)
	n.nodes = make(map[uuid.UUID]*data.Node)
	n.cmdPort = cmdPort
	n.localNodeId = uuid.New()

	return n
}

func (n *NetworkService) Subscribe(subscriber data.ShareSubscriber) {
	n.subscriber = append(n.subscriber, subscriber)
}

func (n NetworkService) SendCommand(cmd data.ShareCommand, node data.Node) {
	// todo: implmenent
}

func (n NetworkService) AddNode(newNode data.Node) {
	n.mu.Lock()
	defer n.mu.Unlock()

	node, ok := n.nodes[newNode.Id()]
	if !ok {
		node = &newNode
		n.nodes[newNode.Id()] = node
	}
	node.SetLastTimeSeen(time.Now().UnixNano())
}

func (n NetworkService) RemoveNode(node data.Node) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// todo: implement, remove node from replica nodes
	delete(n.nodes, node.Id())
}

func (n NetworkService) LocalNodeId() uuid.UUID {
	return n.localNodeId
}

func (n NetworkService) AllNodes() map[uuid.UUID]*data.Node {
	return n.nodes
}

func (n NetworkService) Node(nodeId uuid.UUID) (*data.Node, bool) {
	node, ok := n.nodes[nodeId]
	return node, ok
}

func (n NetworkService) Port() int {
	return n.cmdPort
}

func (n *NetworkService) Start() {
	go n.acceptConnections()
}

func (n NetworkService) Stop() {
	// todo: implement
}

func (n *NetworkService) acceptConnections() {
	cmdSocket, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port()))
	if err != nil {
		log.Fatal(err)
	}
	defer cmdSocket.Close()

	for {
		conn, err := cmdSocket.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go n.handleConnection(conn)
	}
}

func (n *NetworkService) handleConnection(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for {
		cmd, err := readCommand(scanner)
		if err != nil {
			log.Println("Could not read ShareCommand!")
			log.Println(err)
			time.Sleep(2 * time.Second)
			continue
		}
		switch cmd.Type() {
		case data.DownloadRequestCmd:
			log.Println("Received download request")
			n.updateDownloadRequest()
			printCmd(*cmd)
			// todo: implement
		case data.DownloadRequestResultCmd:
			log.Println("Received download request result")
			n.updateDownloadRequestResult()
			printCmd(*cmd)
			// todo: implement
		case data.PushShareListCmd:
			log.Println("Received share list")
			n.updatePushShareList()
			printCmd(*cmd)
			// todo: implement
		default:
			log.Println("Unknown command!")
			time.Sleep(2 * time.Second) // only for debugging
		}
	}
}

func (n NetworkService) updateDownloadRequest() {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		s.DownloadRequest()
	}
}

func (n NetworkService) updateDownloadRequestResult() {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		s.DownloadRequestResult()
	}
}

func (n NetworkService) updatePushShareList() {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		s.PushShareList()
	}
}

// helper func to print command data elements
func printCmd(cmd data.ShareCommand) {
	for i := cmd.Data().Front(); i != nil; i = i.Next() {
		log.Println(i.Value)
	}
}

func readCommand(r *bufio.Scanner) (*data.ShareCommand, error) {
	var line string

	// maybe dont use scanner, just need []byte
	if ok := r.Scan(); ok {
		line = r.Text()
		log.Println("Read line:", line)

		cmd := data.NewShareCommand()
		err := cmd.Deserialize([]byte(line))
		if err != nil {
			return nil, err
		}
		return cmd, nil

	} else {
		if err := r.Err(); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("Could not read command")
}
