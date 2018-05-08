package net

import (
	"bufio"
	"fmt"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"net"
	"testing"
	"time"
)

const (
	cmdPort = 6132
)

func TestAddNode(t *testing.T) {
	node := data.NewNode(uuid.New(), "192.168.1.1")

	networkService := newService()
	networkService.AddNode(*node)

	// check if node got added correctly
	actualNode, ok := networkService.Node(node.Id())
	if !ok {
		t.Error("Could not find added node")
	}
	if node.Id() != actualNode.Id() {
		t.Error("Node id does not match")
	}
	if len(node.Ips()) != len(actualNode.Ips()) {
		t.Error("Ip address count does not match")
	}
	for _, ipExpected := range node.Ips() {
		found := false
		for _, ipActual := range actualNode.Ips() {
			if ipExpected == ipActual {
				found = true
			}
		}
		if !found {
			t.Error("Ip addresses do not match")
		}
	}
}

func TestAddNodeDoubleEntry(t *testing.T) {
	node := data.NewNode(uuid.New(), "192.168.1.1")

	networkService := newService()
	networkService.AddNode(*node)
	networkService.AddNode(*node)

	// check that node got added once, only
	// todo: replace 'len(AllNodes)' with 'NodeCount'
	if len(networkService.AllNodes()) != 1 {
		t.Error("Node count does not match")
	}
}

func TestReceiveShareList(t *testing.T) {
	// todo: implement

}

// positive test
func TestReceiveDownloadRequest(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	fileId := "9ed4ad0c-3dc7-40e9-ab92-0d41a337b829"
	nodeId := "025e3a3d-490c-4c60-84f0-8da54d7d3c67"
	chunkSum := "7bd2b441a33e4d648988638a191f48cf"

	// prepare mocked share subscriber
	finished := make(chan bool)
	s := MockShareSubscriber{
		data.DownloadRequest{fileId, nodeId, chunkSum},
		data.DownloadRequestResult{},
		commonData.SharedFile{},
		t,
		finished,
	}
	// register subscriber
	networkService.Register(s)

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	// prepare shared data
	dlRequest := []byte(fmt.Sprintf("{\"cmd\":\"DOWNLOAD_REQUEST\",\"data\":[{\"fileId\":\"%s\",\"nodeId\":\"%s\",\"chunkChecksum\":\"%s\"}]}", fileId, nodeId, chunkSum))

	conn := newConn(t, cmdPort, 1*time.Second)
	defer conn.Close()

	//n := writeLine(t, conn, b)
	n := writeLine(t, conn, dlRequest)
	if len(dlRequest)+1 != n {
		t.Error("Number of bytes sent do not match")
	}
	// wait until message received and evaluated
	<-finished
}

// negative test, empty download request message
// todo: also implement negative tests for request result and sharedFile
func TestReceiveEmptyDownloadRequest(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	var fileId, nodeId, chunkSum string

	// prepare mocked share subscriber
	finished := make(chan bool)
	s := MockShareSubscriber{
		data.DownloadRequest{fileId, nodeId, chunkSum},
		data.DownloadRequestResult{},
		commonData.SharedFile{},
		t,
		finished,
	}
	// register subscriber
	networkService.Register(s)

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	dlRequest := []byte("{\"cmd\":\"DOWNLOAD_REQUEST\",\"data\":[]}")

	conn := newConn(t, cmdPort, 1*time.Second)
	defer conn.Close()

	n := writeLine(t, conn, dlRequest)
	if len(dlRequest)+1 != n {
		t.Error("Number of bytes sent do not match")
	}
	// wait until message received and evaluated
	select {
	case <-finished:
		t.Error("Subscriber method should not be called!")
	case <-time.After(100 * time.Millisecond):
		// nothing happend, message was empty
	}
}

func TestReceiveMalformedDownloadRequest(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	var fileId, nodeId, chunkSum string

	// prepare mocked share subscriber
	finished := make(chan bool)
	s := MockShareSubscriber{
		data.DownloadRequest{fileId, nodeId, chunkSum},
		data.DownloadRequestResult{},
		commonData.SharedFile{},
		t,
		finished,
	}
	// register subscriber
	networkService.Register(s)

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	dlRequest := []byte("{\"cmd\":\"DOWNLOAD_REQUEST\"}")

	conn := newConn(t, cmdPort, 1*time.Second)
	defer conn.Close()

	n := writeLine(t, conn, dlRequest)
	if len(dlRequest)+1 != n {
		t.Error("Number of bytes sent do not match")
	}
	// wait until message received and evaluated
	select {
	case <-finished:
		t.Error("Subscriber method should not be called!")
	case <-time.After(100 * time.Millisecond):
		// nothing happend, message was malformed
	}
}

func TestReceiveMalformedDownloadRequest_2(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	var fileId, nodeId, chunkSum string

	// prepare mocked share subscriber
	finished := make(chan bool)
	s := MockShareSubscriber{
		data.DownloadRequest{fileId, nodeId, chunkSum},
		data.DownloadRequestResult{},
		commonData.SharedFile{},
		t,
		finished,
	}
	// register subscriber
	networkService.Register(s)

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	dlRequest := []byte(fmt.Sprintf("{\"cmd\":\"DOWNLOAD_REQUEST\",\"data\":[{:\"%s\",\"nodeId\":\"%s\",\"chunkChecksum\":\"%s\"}]}", fileId, nodeId, chunkSum))

	conn := newConn(t, cmdPort, 1*time.Second)
	defer conn.Close()

	n := writeLine(t, conn, dlRequest)
	if len(dlRequest)+1 != n {
		t.Error("Number of bytes sent do not match")
	}
	// wait until message received and evaluated
	select {
	case <-finished:
		t.Error("Subscriber method should not be called!")
	case <-time.After(100 * time.Millisecond):
		// nothing happend, message was malformed
	}
}

func TestReceiveUnknownMessage(t *testing.T) {
	// unkown command type, e.g. not downloadRequest, downloadRequestResult, pushShare,
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	var fileId, nodeId, chunkSum string

	// prepare mocked share subscriber
	finished := make(chan bool)
	s := MockShareSubscriber{
		data.DownloadRequest{fileId, nodeId, chunkSum},
		data.DownloadRequestResult{},
		commonData.SharedFile{},
		t,
		finished,
	}
	// register subscriber
	networkService.Register(s)

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	dlRequest := []byte("{\"cmd\":\"UNRECOGNIZED_CMD_TYPE\",\"data\":[]}")

	conn := newConn(t, cmdPort, 1*time.Second)
	defer conn.Close()

	n := writeLine(t, conn, dlRequest)
	if len(dlRequest)+1 != n {
		t.Error("Number of bytes sent do not match")
	}
	// wait until message received and evaluated
	select {
	case <-finished:
		t.Error("Subscriber method should not be called!")
	case <-time.After(100 * time.Millisecond):
		// nothing happend, unrecognized message type
	}
}

func TestReceiveDownloadRequestResult(t *testing.T) {
	// todo: implement
}

// positive test
func TestSendShareCommand(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	fileId := uuid.New().String()
	localNodeId := networkService.LocalNodeId().String()
	chunkSum := fmt.Sprintf("%x", util.NewHash().Sum(nil))
	cmdData := []interface{}{data.NewDownloadRequest(fileId, localNodeId, chunkSum)}

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	cmd := data.NewShareCommand(data.DownloadRequestCmd, cmdData, destNode.Id(), func() {
		// callback function on send failure
		t.Error("Could not send message")
	})

	// acutally send message
	sender := networkService.Sender()
	select {
	case sender <- *cmd:
	case <-time.After(100 * time.Millisecond):
		t.Error("Sending message timed out")
	}

	// wait whether open connection fails
	time.Sleep(1 * time.Second)
}

// negative test
func TestSendShareCommandUnkownNode(t *testing.T) {
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare download request data
	fileId := uuid.New().String()
	localNodeId := networkService.LocalNodeId().String()
	chunkSum := fmt.Sprintf("%x", util.NewHash().Sum(nil))
	cmdData := []interface{}{data.NewDownloadRequest(fileId, localNodeId, chunkSum)}

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	//networkService.AddNode(*destNode)

	wasCalled := false
	cmd := data.NewShareCommand(data.DownloadRequestCmd, cmdData, destNode.Id(), func() {
		// callback function on send failure
		fmt.Println("Expected call of 'failed to send message callback'")
		wasCalled = true
	})

	// acutally send message
	sender := networkService.Sender()
	select {
	case sender <- *cmd:
	case <-time.After(100 * time.Millisecond):
		t.Error("Sending message timed out")
	}

	// wait whether open connection fails
	time.Sleep(100 * time.Millisecond)
	if !wasCalled {
		t.Error("Callback function for failed cmd sending should have been called!")
	}
}

// negative test
func TestSendShareCommandFailedSerialization(t *testing.T) {
	// todo: implement
	networkService := newService()
	networkService.Start()
	defer networkService.Stop()

	// prepare destination node
	destNode := data.NewNode(uuid.New(), "localhost")
	networkService.AddNode(*destNode)

	// create cmd with unknown command type '-1'
	wasCalled := false
	cmd := data.NewShareCommand(-1, nil, destNode.Id(), func() {
		// callback function on send failure
		fmt.Println("Expected call of 'failed to send message callback'")
		wasCalled = true
	})

	// acutally send message
	sender := networkService.Sender()
	select {
	case sender <- *cmd:
	case <-time.After(100 * time.Millisecond):
		t.Error("Sending message timed out")
	}

	// wait whether open connection fails
	time.Sleep(100 * time.Millisecond)
	if !wasCalled {
		t.Error("Callback function for failed cmd sending should have been called!")
	}
}

func readLine(t *testing.T, conn net.Conn) string {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Error(err)
	}

	return line
}

func writeLine(t *testing.T, conn net.Conn, b []byte) int {
	n, err := conn.Write(b)
	if err != nil {
		t.Error(err)
		return -1
	}
	_, err = conn.Write([]byte{'\n'})
	if err != nil {
		t.Error(err)
		return -1
	}
	return n + 1
}

func newConn(t *testing.T, port int, timeout time.Duration) net.Conn {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		t.Error(err)
	}
	return conn
}

func newListener(t *testing.T, port int, msgCount int) net.Conn {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Error(err)
	}
	var server net.Conn
	go func() {
		defer ln.Close()
		for i := 0; i < msgCount; i++ {
			fmt.Println("before")
			server, err = ln.Accept()
			fmt.Println("after")
			if err != nil {
				t.Error(err)
				break
			}
		}
	}()
	time.Sleep(1 * time.Second)
	fmt.Println(server)
	return server
}

func newService() *NetworkService {
	localId := uuid.New()
	networkService := NewNetworkService(localId, cmdPort)

	return networkService
}

type MockShareSubscriber struct {
	expectedDownloadRequest       data.DownloadRequest
	expectedDownloadRequestResult data.DownloadRequestResult
	expectedSharedFile            commonData.SharedFile
	t                             *testing.T
	finished                      chan bool
}

func (m MockShareSubscriber) ReceivedDownloadRequest(actualRequest data.DownloadRequest) {
	expectedRequest := m.expectedDownloadRequest

	if expectedRequest.FileId() != actualRequest.FileId() {
		m.t.Error("Download request file id does not match")
	}
	if expectedRequest.NodeId() != actualRequest.NodeId() {
		m.t.Error("Download request node id does not match")
	}
	if expectedRequest.ChunkChecksum() != actualRequest.ChunkChecksum() {
		m.t.Error("Download request chunk checksum does not match")
	}
	m.finished <- true
}

func (m MockShareSubscriber) ReceivedDownloadRequestResult(actualRequestResult data.DownloadRequestResult) {
	expectedRequestResult := m.expectedDownloadRequestResult

	if expectedRequestResult.FileId() != actualRequestResult.FileId() {
		m.t.Error("Download request result file id does not match")
	}
	if expectedRequestResult.NodeId() != actualRequestResult.NodeId() {
		m.t.Error("Download request result node id does not match")
	}
	if expectedRequestResult.ChunkChecksum() != actualRequestResult.ChunkChecksum() {
		m.t.Error("Download request result chunk checksum does not match")
	}
	if expectedRequestResult.DownloadPort() != actualRequestResult.DownloadPort() {
		m.t.Error("Download request result download port does not match")
	}
	m.finished <- true
}

func (m MockShareSubscriber) ReceivedShareList(sf commonData.SharedFile) {
	m.finished <- true
}
