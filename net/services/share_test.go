package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	infoPeriod   = 500 * time.Millisecond
	maxUploads   = 5
	maxDownloads = 5
)

func TestReceivedDownloadRequestDenyUploadFileNotShared(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	sender := make(chan data.ShareCommand)

	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)

	// prepare download request of unknown file
	fileId := uuid.New().String()
	nodeId := uuid.New().String()
	chunkChecksum := fmt.Sprintf("%x", util.NewHash().Sum(nil))
	request := data.NewDownloadRequest(fileId, nodeId, chunkChecksum)

	// start message reader for deny message for unknown files
	done := make(chan bool)
	go readDenyUpload(t, done, sender, request)

	// call receivedDownloadRequest method
	shareService.ReceivedDownloadRequest(*request)

	// wait for message
	<-done
}

func TestReceivedDownloadRequestDenyUploadChunkUnknown(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	sender := make(chan data.ShareCommand)

	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)

	// prepare shared file
	tmpFile := createFile(t, base)
	defer os.Remove(tmpFile.Name())

	relativePath, err := filepath.Rel(base, tmpFile.Name())
	if err != nil {
		t.Error(err)
	}
	meta := commonData.NewFileMetadata(tmpFile.Name(), relativePath)
	sf := commonData.NewSharedFile(meta)
	shareService.AddLocalSharedFile(*sf)

	// prepare download request of unknown chunk
	nodeId := uuid.New().String()
	chunkChecksum := fmt.Sprintf("%x", util.NewHash().Sum(nil))
	request := data.NewDownloadRequest(sf.FileId(), nodeId, chunkChecksum)

	// start message reader for deny message for unknown files
	done := make(chan bool)
	go readDenyUpload(t, done, sender, request)

	// call receivedDownloadRequest method
	shareService.ReceivedDownloadRequest(*request)

	// wait for message
	<-done
}

func TestReceivedDownloadRequestDenyUploadChunkNotLocal(t *testing.T) {
	// todo: implement
	// do not calculate chunks on fileMetadata create automatically
	// move it to own exported function, maybe this function accepts
	// a callback on calculation finish, or make the function call
	// blocking and  accept 'done' channel
	// done := make(chan bool)
	// go meta.CalculateChecksums(done)
	// done <-
	// meta.CalculateChecksums(func() {fmt.Println("done")})
}

func TestReceivedDownloadRequestResultAcceptUploadLocalFileSharedDirectory(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	sender := make(chan data.ShareCommand)

	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)

	// prepare shared file
	tmpFile := createFile(t, base)
	defer os.Remove(tmpFile.Name())

	relativePath, err := filepath.Rel(base, tmpFile.Name())
	if err != nil {
		t.Error(err)
	}
	meta := commonData.NewFileMetadata(tmpFile.Name(), relativePath)
	sf := commonData.NewSharedFile(meta)
	shareService.AddLocalSharedFile(*sf)
	// wait for chunk calculation
	time.Sleep(100 * time.Millisecond)

	// prepare download request of unknown chunk
	nodeId := uuid.New().String()
	chunkChecksum := sf.LocalChunksChecksums()[0]
	request := data.NewDownloadRequest(sf.FileId(), nodeId, chunkChecksum)

	// start message reader for message
	done := make(chan bool)
	go acceptUpload(t, done, sender)

	// call receivedDownloadRequest method
	shareService.ReceivedDownloadRequest(*request)

	// wait for message
	<-done
}

func acceptUpload(t *testing.T, done chan bool, sender chan data.ShareCommand) {
	var cmd data.ShareCommand
	select {
	case cmd = <-sender:
	case <-time.After(500 * time.Millisecond):
		t.Error("Did not receive upload message, timed out")
	}
	// check share cmd type
	if cmd.Type() != data.DownloadRequestResultCmd {
		t.Error("Wrong share command type")
	}
	for _, d := range cmd.Data() {
		// get data
		requestResult := d.(*data.DownloadRequestResult)
		// connecto to given port
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", requestResult.DownloadPort()))
		if err != nil {
			t.Error(err)
		}
		// create buffer
		buf := make([]byte, 0, 4096)
		chunkData := bytes.NewBuffer(buf)
		if err != nil {
			t.Error(err)
		}
		// create multi writer, to create hash simultaniously
		hash := util.NewHash()
		multi := io.MultiWriter(chunkData, hash)
		// copy data
		io.Copy(multi, conn)

		// check if data was correctly transmitted
		actualChunkChecksum := fmt.Sprintf("%x", hash.Sum(nil))
		log.Println("expected:", requestResult.ChunkChecksum(), "actual:", actualChunkChecksum)
		if requestResult.ChunkChecksum() != actualChunkChecksum {
			t.Error("Chunk data was not correctly transfered")
		}
	}

	done <- true
}

func TestReceivedDownloadRequestResultAcceptUploadLocalFileDownloadDirectory(t *testing.T) {
	// todo: implement
	// upload chunk of file which is completely downloaded, from download directory
}

func TestReceivedDownloadRequestResultAcceptUploadPartialFileDownloadDirectory(t *testing.T) {
	// todo: implement
	// upload chunk of file which is not local yet, only partially downloaded, from download directory
}

//func TestReceivedShareListFirstTimeSeen(t *testing.T) {
//	// todo: implement
//	// received shared info the first time
//
//	// prepare share service parameter
//	localNodeId := uuid.New()
//	sender := make(chan data.ShareCommand)
//
//	downloadDir, base := prepareDirs(t)
//	defer os.RemoveAll(downloadDir)
//	defer os.RemoveAll(base)
//
//	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)
//
//	// add node
//	node := data.NewNode(uuid.New(), "192.168.1.1")
//	shareService.AddNode(*node)
//	log.Println("Node added to service:", node.Id())
//	log.Println("Local node id:", localNodeId)
//
//	// prepare share file info
//	tmpFile := createFile(t, base)
//	defer os.Remove(tmpFile.Name())
//	relativePath, err := filepath.Rel(base, tmpFile.Name())
//	if err != nil {
//		t.Error(err)
//	}
//	meta := commonData.NewFileMetadata(tmpFile.Name(), relativePath)
//	sf := commonData.NewSharedFile(meta)
//
//	// wait for chunk calculation
//	time.Sleep(100 * time.Millisecond)
//
//	// add replica node
//	replicaNode := commonData.NewReplicaNode(node.Id(), sf.ChunkSums(), false)
//	sf.UpdateReplicaNode(replicaNode)
//
//	// marshalling and unmarshalling required to get correct object state
//	marshalled, _ := json.Marshal(sf)
//	var remoteSf commonData.SharedFile
//	json.Unmarshal(marshalled, &remoteSf)
//	log.Println("ReplicaNodeCount:", len(remoteSf.ReplicaNodes()))
//	for _, replicaNode := range remoteSf.ReplicaNodes() {
//		log.Println("replicaNode:", replicaNode.Id())
//		log.Println("replicaNode:", replicaNode.ChunkCount())
//	}
//	// receive share list from other client
//	shareService.ReceivedShareList(remoteSf)
//
//	// start reader for share info message
//	done := make(chan bool)
//	go readShareInfoMsgRemoteInfo(t, done, sender, sf)
//
//	// start share service
//	shareService.Start()
//	defer shareService.Stop()
//
//	// wait for message read
//	<-done
//}
//
//func readShareInfoMsgRemoteInfo(t *testing.T, done chan bool, sender chan data.ShareCommand, sf *commonData.SharedFile) {
//	for {
//		var cmd data.ShareCommand
//		select {
//		case cmd = <-sender:
//		case <-time.After(1500 * time.Millisecond):
//			t.Error("Expected to receive share info message, timed out")
//		}
//		log.Println("Cmd:", cmd)
//		// check cmd type
//		if cmd.Type() != data.PushShareListCmd {
//			// ignore other message types
//			continue
//		}
//		// handle cmd data
//		for _, d := range cmd.Data() {
//			actualSf := d.(commonData.SharedFile)
//			log.Println("ReplicaNodeCount:", len(actualSf.ReplicaNodes()))
//			log.Println("actualData:", actualSf.Chunks()[0].IsLocal())
//			for _, replicaNode := range actualSf.ReplicaNodes() {
//				log.Println("replicaNode:", replicaNode.Id())
//				log.Println("replicaNode:", replicaNode.ChunkCount())
//			}
//		}
//		// exit loop
//		break
//	}
//	// finish reading
//	done <- true
//}

func TestSendingCompleteMsg(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	sender := make(chan data.ShareCommand)

	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)

	// prepare local file, so that share information get send to other nodes
	testFile := createFile(t, base)
	defer os.Remove(testFile.Name())
	relativePath, err := filepath.Rel(base, testFile.Name())
	if err != nil {
		t.Error(err)
	}
	// add node to share service
	node := data.NewNode(uuid.New(), "192.168.1.1")
	shareService.AddNode(*node)
	// create metadata, calculate chunks
	meta := commonData.NewFileMetadata(testFile.Name(), relativePath)
	sf := commonData.NewSharedFile(meta)
	// wait for chunk calculation to finish
	for len(sf.Checksum()) <= 0 {
		time.Sleep(5 * time.Millisecond)
	}
	// add any replica node to shared file
	expectedReplicaNode := commonData.NewReplicaNode(node.Id(), sf.LocalChunksChecksums(), len(sf.Checksum()) > 0)
	sf.UpdateReplicaNode(expectedReplicaNode)
	shareService.AddLocalSharedFile(*sf)

	// start reader for messages
	done := make(chan bool)
	go readCompleteMsg(t, done, 2, sender, *node, localNodeId)

	// start share info service
	shareService.Start()
	defer shareService.Stop()

	// wait for messages
	<-done
}

func TestAddNodeAddShareInfo(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	sender := make(chan data.ShareCommand)

	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	shareService := NewShareService(localNodeId, sender, infoPeriod, downloadDir, maxUploads, maxDownloads)

	// add local file, so that share information get send to other nodes
	testFile := createFile(t, base)
	defer os.Remove(testFile.Name())
	relativePath, err := filepath.Rel(base, testFile.Name())
	if err != nil {
		t.Error(err)
	}
	meta := commonData.NewFileMetadata(testFile.Name(), relativePath)
	sf := commonData.NewSharedFile(meta)
	shareService.AddLocalSharedFile(*sf)

	// add node
	node := data.NewNode(uuid.New(), "192.168.1.1")
	shareService.AddNode(*node)
	nodes := []data.Node{*node}

	// wait for chunk calculation to finish
	for len(sf.Checksum()) <= 0 {
		time.Sleep(5 * time.Millisecond)
	}

	// start reader for messages
	done := make(chan bool)
	go readShareInfoMsg(t, done, 2, sender, []commonData.SharedFile{*sf}, nodes, localNodeId)

	// start share info service
	shareService.Start()
	defer shareService.Stop()

	// wait for messages
	<-done

	// add new node
	node2 := data.NewNode(uuid.New(), "192.168.1.2")
	shareService.AddNode(*node2)
	nodes = []data.Node{*node, *node2}

	// start reader for messages
	go readShareInfoMsg(t, done, 4, sender, []commonData.SharedFile{*sf}, nodes, localNodeId)

	// wait for messages
	<-done

	// add new shared file
	testFile2 := createFile(t, base)
	defer os.Remove(testFile2.Name())
	relativePath2, err := filepath.Rel(base, testFile2.Name())
	if err != nil {
		t.Error(err)
	}
	meta2 := commonData.NewFileMetadata(testFile2.Name(), relativePath2)
	sf2 := commonData.NewSharedFile(meta2)
	shareService.AddLocalSharedFile(*sf2)

	// wait for chunk calculation to finish
	for len(sf.Checksum()) <= 0 {
		time.Sleep(5 * time.Millisecond)
	}

	// start reader for messages
	go readShareInfoMsg(t, done, 8, sender, []commonData.SharedFile{*sf, *sf2}, nodes, localNodeId)

	// wait for messages
	<-done
}

func readShareInfoMsg(t *testing.T, done chan bool, msgCount int, sender chan data.ShareCommand, expectedSharedFiles []commonData.SharedFile, nodes []data.Node, localNodeId uuid.UUID) {
	// read messages
Loop:
	for i := 0; i < msgCount; i++ {
		// check whether share info messages arrive before 'infoPeriod' times out
		var cmd data.ShareCommand
		select {
		case cmd = <-sender:
		case <-time.After(infoPeriod + 100*time.Millisecond):
			t.Error("Expected share info to arrive periodically")
			break Loop
		}
		// check cmd type
		if cmd.Type() != data.PushShareListCmd {
			t.Error("Wrong share command type")
		}
		// check if message was sent to one of our defined nodes
		// consider random order of receiving nodes, because map[y]x returns items randomly
		foundNode := false
		for _, n := range nodes {
			if n.Id() == cmd.Destination() {
				foundNode = true
				break
			}
		}
		if !foundNode {
			t.Error("Received information for wrong node")
		}
		// check replica node information
		for _, data := range cmd.Data() {
			sfActual := data.(commonData.SharedFile)

			// shared file must contain sender as replica node
			replicaNode, ok := sfActual.ReplicaNodeById(localNodeId)
			if !ok {
				t.Error("Could not find local node as replica node")
			}
			// replica node information must be complete
			if !replicaNode.IsAllInfoReceived() {
				t.Error("Replica node must be complete, was node for local file")
			}
			// replica node's share info state must be active
			if replicaNode.IsCompleteMsgSent() {
				t.Error("Share info should not have been stopped yet")
			}

			// check if correct shared file info was sent
			foundExpectedFile := false
			for _, sf := range expectedSharedFiles {
				if sf.FileId() == sfActual.FileId() {
					foundExpectedFile = true
					// replica node must contain chunk information of shared file
					for _, chunkSum := range sf.ChunkSums() {
						if !replicaNode.Contains(chunkSum) {
							t.Errorf("Replica node is missing chunk: '%s'\n", chunkSum)
							break
						}
					}
				}
			}
			if !foundExpectedFile {
				t.Error("Shared file info is missing")
			}
		}
	}
	done <- true
}

func readCompleteMsg(t *testing.T, done chan bool, msgCount int, sender chan data.ShareCommand, completeNode data.Node, localNodeId uuid.UUID) {
	// read messages
Loop:
	for i := 0; i < msgCount; i++ {
		// check whether share info messages arrive before 'infoPeriod' times out
		var cmd data.ShareCommand
		select {
		case cmd = <-sender:
		case <-time.After(infoPeriod + 100*time.Millisecond):
			log.Println("Expected no more message, becaue complete message get only sent once")
			break Loop
		}
		// check cmd type
		if cmd.Type() != data.PushShareListCmd {
			t.Error("Wrong share command type")
		}
		// check replica node information
		for _, data := range cmd.Data() {
			sfActual := data.(commonData.SharedFile)

			// shared file must contain sender as replica node
			replicaNodeSender, ok := sfActual.ReplicaNodeById(localNodeId)
			if !ok {
				t.Error("Could not find local node as replica node")
			}
			// replica node does not share chunk information in 'complete msg'
			if replicaNodeSender.ChunkCount() > 0 {
				t.Error("'Complete message' is invalid")
			}
			// replica node information must be complete
			if !replicaNodeSender.IsAllInfoReceived() {
				t.Error("Replica node must be complete for sending 'complete msg'")
			}
		}
	}
	done <- true
}

func TestRemoveNode(t *testing.T) {
	// todo: implement
}

func TestAddLocalSharedFile(t *testing.T) {
	// todo: implement
}
