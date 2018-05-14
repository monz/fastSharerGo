package net

import (
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/net/data"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// deny upload because can not acquire upload token
func TestUploadCannotAcquireToken(t *testing.T) {
	// prepare uploader parameter
	localNodeId := uuid.New()
	senderChan := make(chan data.ShareCommand)
	sender := NewShareSender(senderChan)

	// create uploader
	maxUploads := 0
	uploader := NewShareUploader(localNodeId, maxUploads, sender)

	// prepare dirs
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	// prepare shared file
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())

	// prepare download request of unknown chunk
	nodeId := uuid.New().String()
	chunkChecksum := sf.LocalChunksChecksums()[0]
	request := data.NewDownloadRequest(sf.FileId(), nodeId, chunkChecksum)

	// start message reader for deny message
	done := make(chan bool)
	go readDenyUpload(t, done, senderChan, request)

	// start upload
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done
}

// helper func
func readDenyUpload(t *testing.T, done chan bool, sender chan data.ShareCommand, request *data.DownloadRequest) {
	// read data or time out
	var cmd data.ShareCommand
	select {
	case cmd = <-sender:
	case <-time.After(6 * time.Second): // must be longer than 'upload token' acquire timeout
		t.Error("Did not receive denied upload message, timed out")
	}
	// check data type
	if cmd.Type() != data.DownloadRequestResultCmd {
		t.Error("Wrong share command type")
	}
	// check data
	for _, d := range cmd.Data() {
		requestResult := d.(*data.DownloadRequestResult)
		// check if request was denied, expecting '-1' port
		if requestResult.DownloadPort() != -1 {
			t.Error("Got port nuber, but expected '-1'")
		}
		// check if denied right request
		if request.FileId() != requestResult.FileId() {
			t.Error("Request result file id does not match")
		}
		if request.ChunkChecksum() != requestResult.ChunkChecksum() {
			t.Error("Request result chunk checksum does not match")
		}
	}
	done <- true
}

// accept upload, but fail to send request result message to other client
// subsequently upload requests should get accepted and handled properly
func TestUploadAcceptFailedSendingResult(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	senderChan := make(chan data.ShareCommand)
	sender := NewShareSender(senderChan)

	// prepare dirs
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	maxUploads := 1
	uploader := NewShareUploader(localNodeId, maxUploads, sender)

	// prepare shared file
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())

	// prepare download request of unknown chunk
	nodeId := uuid.New().String()
	chunkChecksum := sf.LocalChunksChecksums()[0]

	// start message reader for message
	done := make(chan bool)
	go failSendingUploadAccept(t, done, senderChan)

	// call receivedDownloadRequest method
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done

	// start message reader for message
	// expect download request result message with valid port number
	go uploadAccept(t, done, senderChan)

	// check wether upload token was released and subsequent requests get handled
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done
}

// helper func
func failSendingUploadAccept(t *testing.T, done chan bool, sender chan data.ShareCommand) {
	// read data or time out
	var cmd data.ShareCommand
	select {
	case cmd = <-sender:
		// let upload fail
		cmd.Callback()
	case <-time.After(socketTimeout + 100*time.Millisecond):
		t.Error("Did not receive upload message, timed out")
	}

	done <- true
}

// helper func
func uploadAccept(t *testing.T, done chan bool, sender chan data.ShareCommand) {
	// read data or time out
	var cmd data.ShareCommand
	select {
	case cmd = <-sender:
		if cmd.Type() != data.DownloadRequestResultCmd {
			t.Error("Expected downloadRequestResult")
		}
		for _, d := range cmd.Data() {
			requestResult := d.(*data.DownloadRequestResult)
			if requestResult.DownloadPort() <= 0 {
				t.Error("Expected that upload would be accepted")
			}
		}
	case <-time.After(socketTimeout + 100*time.Millisecond):
		t.Error("Did not receive upload message, timed out")
	}
	done <- true
}

// download request contains invalid node id, cannot parse
// test whether subsequent download requests get handled properly
func TestUploadAcceptFailedParseNodeId(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	senderChan := make(chan data.ShareCommand)
	sender := NewShareSender(senderChan)

	// prepare dirs
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	// create uploader
	maxUploads := 1
	uploader := NewShareUploader(localNodeId, maxUploads, sender)

	// prepare shared file
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())

	// prepare download request of unknown chunk
	nodeId := "" // invalid id
	chunkChecksum := sf.LocalChunksChecksums()[0]

	// call receivedDownloadRequest method, will fail to parse node id, upload token should get released
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// start message reader for message
	done := make(chan bool)
	go uploadAccept(t, done, senderChan)

	// check wether upload token was released and subsequent requests get handled
	// start upload, expect download request result message with valid download port
	nodeId = uuid.New().String()
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done
}

// other client requests download, gets request result but do
// not connect in time, so connection times out
// test whether subsequent requests get handled normally
func TestUploadAcceptFailedClientConnectTimeout(t *testing.T) {
	// prepare share service parameter
	localNodeId := uuid.New()
	senderChan := make(chan data.ShareCommand)
	sender := NewShareSender(senderChan)

	// prepare dirs
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)

	maxUploads := 1
	uploader := NewShareUploader(localNodeId, maxUploads, sender)

	// prepare shared file
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())

	// prepare download request of unknown chunk
	nodeId := uuid.New().String()
	chunkChecksum := sf.LocalChunksChecksums()[0]

	// start message reader for message
	done := make(chan bool)
	go failConnectClient(t, done, senderChan)

	// call receivedDownloadRequest method
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done

	// start message reader for message
	go uploadAccept(t, done, senderChan)

	// check wether upload token was released and subsequent requests get handled
	uploader.Upload(sf, chunkChecksum, nodeId, filepath.Join(downloadDir, sf.FileRelativePath()))

	// wait for message
	<-done
}

// helper func
func failConnectClient(t *testing.T, done chan bool, sender chan data.ShareCommand) {
	// read data or time out
	var cmd data.ShareCommand
	select {
	case cmd = <-sender:
		// check cmd type
		if cmd.Type() != data.DownloadRequestResultCmd {
			t.Error("Wrong share command type")
		}
		// let open connection time out on other client
		time.Sleep(socketTimeout + 100*time.Millisecond)
	case <-time.After(500 * time.Millisecond):
		t.Error("Did not receive upload message, timed out")
	}

	done <- true
}
