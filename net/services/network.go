package net

import (
        "fmt"
        "log"
        "sync"
        "time"
        "net"
        "github.com/monz/fastSharerGo/net/data"
        "github.com/google/uuid"
)

const (
        socketTiemout = time.Second * 5
)

type Network struct {
        nodes map[uuid.UUID]*data.Node
        cmdPort int
        localNodeId uuid.UUID
        mu sync.Mutex
}

func NewNetwork(cmdPort int) *Network {
        n := new(Network)
        n.nodes = make(map[uuid.UUID]*data.Node)
        n.cmdPort = cmdPort
        n.localNodeId = uuid.New()

        return n
}

func (n Network) SendCommand(cmd data.ShareCommand, node data.Node) {
        // todo: implmenent
}

func (n Network) AddNode(newNode data.Node) {
        n.mu.Lock()
        defer n.mu.Unlock()

        node, ok := n.nodes[newNode.Id()]
        if !ok {
                node = &newNode
                n.nodes[newNode.Id()] = node
        }
        node.SetLastTimeSeen(time.Now().UnixNano())
}

func (n Network) RemoveNode(node data.Node) {
        n.mu.Lock()
        defer n.mu.Unlock()

        // todo: implement, remove node from replica nodes
        delete(n.nodes, node.Id())
}

func (n Network) LocalNodeId() uuid.UUID {
        return n.localNodeId
}

func (n Network) AllNodes() map[uuid.UUID]*data.Node {
        return n.nodes
}

func (n Network) Node(nodeId uuid.UUID) (*data.Node, bool) {
        node, ok := n.nodes[nodeId]
        return node, ok
}

func (n Network) Start() {
        go acceptConnections(n.cmdPort)
}

func (n Network) Stop() {
        // todo: implement
}

func acceptConnections(cmdPort int) {
        cmdSocket, err := net.Listen("tcp", fmt.Sprintf(":%d", cmdPort))
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
                go handleConnection(&conn)
        }
}

func handleConnection(conn *net.Conn) {
        for {
            cmd, ok := readCommand(conn)
            if !ok {
                    log.Println("Could not read ShareCommand!")
                    continue
            }
            switch cmd.Type() {
            case data.DownloadRequestCmd:
                    log.Println("Received download request")
            case data.DownloadRequestResultCmd:
                    log.Println("Received download request result")
            case data.PushShareListCmd:
                    log.Println("Received share list")
            default:
                    log.Println("Unknown command!")
            }
        }
}

func readCommand(conn *net.Conn) (data.ShareCommand, bool) {
    return nil, false
}
