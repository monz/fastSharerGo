package net

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
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
	cmdSocket   net.Listener
	stopped     bool
	mu          sync.Mutex
}

func NewNetworkService(localId uuid.UUID, cmdPort int) *NetworkService {
	n := new(NetworkService)
	n.nodes = make(map[uuid.UUID]*data.Node)
	n.cmdPort = cmdPort
	n.localNodeId = localId
	n.sender = make(chan data.ShareCommand)
	n.stopped = false

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
		log.Printf("Send command: '%s'", data)
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

func (n *NetworkService) Stop() {
	n.stopped = true
	if n.cmdSocket != nil {
		n.cmdSocket.Close()
	}
}

func (n *NetworkService) acceptConnections() {
	var err error
	n.cmdSocket, err = net.Listen("tcp", fmt.Sprintf(":%d", n.Port()))
	if err != nil {
		log.Fatal(err)
	}
	// connection gets closed in 'Stop' function

	for !n.stopped {
		conn, err := n.cmdSocket.Accept()
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
		cmd, ok, err := readCommand(scanner)
		if err != nil {
			log.Println("Could not read ShareCommand!")
			log.Println(err)
			if !ok {
				conn.Close()
				break
			}
			continue
		}
		switch cmd.Type() {
		case data.DownloadRequestCmd:
			log.Println("Received download request from other client")
			n.receivedDownloadRequest(cmd.Data())
			printCmd(*cmd)
		case data.DownloadRequestResultCmd:
			log.Println("Received download request result from other client")
			n.receivedDownloadRequestResult(cmd.Data())
			printCmd(*cmd)
		case data.PushShareListCmd:
			log.Println("Received share list from other client")
			n.receivedShareList(cmd.Data())
			printCmd(*cmd)
		default:
			log.Println("Unknown command!")
		}
	}
}

func (n NetworkService) receivedDownloadRequest(l []interface{}) {
	for _, s := range n.subscriber {
		for _, v := range l {
			s.ReceivedDownloadRequest(v.(data.DownloadRequest))
		}
	}
}

func (n NetworkService) receivedDownloadRequestResult(l []interface{}) {
	for _, s := range n.subscriber {
		for _, v := range l {
			s.ReceivedDownloadRequestResult(v.(data.DownloadRequestResult))
		}
	}
}

func (n NetworkService) receivedShareList(l []interface{}) {
	for _, s := range n.subscriber {
		for _, v := range l {
			s.ReceivedShareList(v.(commonData.SharedFile))
		}
	}
}

// helper func to print command data elements
func printCmd(cmd data.ShareCommand) {
	for _, v := range cmd.Data() {
		log.Println(v)
	}
}

// returns read shareCommand, scanner still holds data and error
// if reached EOF, returns shareCommand=nil, moreData=false, error=EOF
func readCommand(r *bufio.Scanner) (*data.ShareCommand, bool, error) {
	var line string

	// maybe dont use scanner, just need []byte
	if ok := r.Scan(); ok {
		line = r.Text()
		log.Println("Read line:", line)

		cmd := data.NewEmptyShareCommand()
		err := data.DeserializeShareCommand([]byte(line), cmd)
		if err != nil {
			return nil, ok, err
		}
		return cmd, ok, nil

	} else {
		if err := r.Err(); err != nil {
			return nil, ok, err
		}
	}
	return nil, false, errors.New("Reached EOF")
}
