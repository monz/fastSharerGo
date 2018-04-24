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
	socketTimeout = time.Second * 5
)

type NetworkService struct {
	nodes       map[uuid.UUID]*data.Node
	cmdPort     int
	localNodeId uuid.UUID
	subscriber  []data.ShareSubscriber
	sender      chan data.ShareCommand
	mu          sync.Mutex
}

func NewNetworkService(cmdPort int) *NetworkService {
	n := new(NetworkService)
	n.nodes = make(map[uuid.UUID]*data.Node)
	n.cmdPort = cmdPort
	n.localNodeId = uuid.New()
	n.sender = make(chan data.ShareCommand)

	return n
}

func (n *NetworkService) Register(subscriber data.ShareSubscriber) {
	n.subscriber = append(n.subscriber, subscriber)
}

func (n *NetworkService) AddNode(newNode data.Node) {
	n.mu.Lock()
	defer n.mu.Unlock()

	node, ok := n.nodes[newNode.Id()]
	if !ok {
		node = &newNode
		n.nodes[newNode.Id()] = &newNode
	}
	node.SetLastTimeSeen(time.Now().UnixNano())
}

func (n *NetworkService) RemoveNode(node data.Node) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// todo: implement, remove node from replica nodes
	delete(n.nodes, node.Id())
}

func (n *NetworkService) Sender() chan data.ShareCommand {
	return n.sender
}

func (n *NetworkService) sendCommand(cmd data.ShareCommand) {
	node, ok := n.nodes[cmd.Destination()]
	if !ok {
		log.Printf("Node '%s' not found\n", cmd.Destination())
		cmd.Callback()
		return
	}

	successfullySend := false
	for _, ip := range node.Ips() {
		// connect to node, or use existing connection
		s, err := node.Connect(ip, n.cmdPort, socketTimeout)
		if err != nil {
			log.Println(err)
			continue
		}
		// send share command
		log.Printf("Send command: '%v'", cmd)
		data, err := data.SerializeShareCommand(cmd)
		if err != nil {
			log.Println(err)
			break
		}
		_, err = s.Write(data)
		if err != nil {
			log.Println(err)
			continue
		}
		_, err = s.Write([]byte{'\n'})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println("Command was actually sent!")
		successfullySend = true
	}
	// remove node from list if could not send message successfully
	if !successfullySend {
		n.RemoveNode(*node)
		cmd.Callback()
	}
}

func (n *NetworkService) send() {
	// get share command, is blocking
	for {
		log.Println("Ready for receiving messages to send")
		cmd := <-n.sender
		log.Println("Received message to send, sending...")
		go n.sendCommand(cmd)
	}
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
	go n.send()
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
			log.Println("Received download request from other client")
			n.receivedDownloadRequest()
			printCmd(*cmd)
			// todo: implement
		case data.DownloadRequestResultCmd:
			log.Println("Received download request result from other client")
			n.receivedDownloadRequestResult()
			printCmd(*cmd)
			// todo: implement
		case data.PushShareListCmd:
			log.Println("Received share list from other client")
			n.receivedShareList(cmd.Data())
			printCmd(*cmd)
			// todo: implement
		default:
			log.Println("Unknown command!")
			time.Sleep(2 * time.Second) // only for debugging
		}
	}
}

func (n NetworkService) receivedDownloadRequest() {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		s.ReceivedDownloadRequest()
	}
}

func (n NetworkService) receivedDownloadRequestResult() {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		s.ReceivedDownloadRequestResult()
	}
}

func (n NetworkService) receivedShareList(l []interface{}) {
	log.Println("Current subscriber:", n.subscriber)
	for _, s := range n.subscriber {
		// update subscriber for all data list entries
		for _, v := range l {
			s.ReceivedShareList(v.(data.SharedFile))
		}
	}
}

// helper func to print command data elements
func printCmd(cmd data.ShareCommand) {
	for _, v := range cmd.Data() {
		log.Println(v)
	}
}

func readCommand(r *bufio.Scanner) (*data.ShareCommand, error) {
	var line string

	// maybe dont use scanner, just need []byte
	if ok := r.Scan(); ok {
		line = r.Text()
		log.Println("Read line:", line)

		cmd := data.NewEmptyShareCommand()
		err := data.DeserializeShareCommand([]byte(line), cmd)
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
