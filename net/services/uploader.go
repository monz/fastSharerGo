package net

import (
	"errors"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type Uploader interface {
	Upload(sf *commonData.SharedFile, chunkChecksum string, nodeId string, filePath string)
	Fail(fileId string, chunkChecksum string, nodeId string)
}

type ShareUploader struct {
	localNodeId  uuid.UUID
	uploadTokens chan int
	sender       Sender
	mu           sync.Mutex
}

func NewShareUploader(localNodeId uuid.UUID, uploadTokens int, sender Sender) *ShareUploader {
	s := new(ShareUploader)
	s.localNodeId = localNodeId
	s.uploadTokens = util.InitSema(uploadTokens)
	s.sender = sender

	return s
}

func (s *ShareUploader) Upload(sf *commonData.SharedFile, chunkChecksum string, nodeId string, filePath string) {
	log.Println("Waiting to acquire upload token...")
	select {
	case <-s.uploadTokens:
		log.Println("Could acquire upload token")
		s.accept(sf, chunkChecksum, nodeId, filePath)
	case <-time.After(tokenAcquireTimeout):
		s.deny(sf.FileId(), chunkChecksum, nodeId)
		return
	}
}

func (s *ShareUploader) accept(sf *commonData.SharedFile, chunkChecksum string, nodeId string, downloadPath string) {
	log.Println("Accept download request")
	// open random port
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// send port information to other client
	requestResult := []interface{}{data.NewDownloadRequestResult(sf.FileId(), s.localNodeId.String(), chunkChecksum, l.Addr().(*net.TCPAddr).Port)}
	id, err := uuid.Parse(nodeId)
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	s.sender.SendCallback(data.DownloadRequestResultCmd, requestResult, id, func() {
		// could not send data
		log.Println("Could not send download request result!")
		s.uploadFail()
		return
	})
	// wait for other client to connect then upload chunk data
	// set timeout
	err = l.SetDeadline(time.Now().Add(socketTimeout))
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// accept connection
	conn, err := l.AcceptTCP()
	if err != nil {
		if err, ok := err.(*net.OpError); ok && err.Timeout() {
			// connection timed out
			log.Println("Connection timed out, other client did not connect within given time!")
		}
		log.Println(err)
		s.uploadFail()
		return
	}
	defer conn.Close()
	// remote client accepted connection, disable timeout/deadline
	l.SetDeadline(time.Time{})
	// send data
	err = s.sendData(sf, conn, sf.FileId(), chunkChecksum, downloadPath)
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// release upload token
	s.uploadSuccess()
}

func (s *ShareUploader) deny(fileId string, chunkChecksum string, nodeId string) {
	log.Println("Cannot accept download request")
	requestResult := []interface{}{data.NewDownloadRequestResult(fileId, s.localNodeId.String(), chunkChecksum, data.DenyDownload)}
	id, err := uuid.Parse(nodeId)
	if err != nil {
		log.Println(err)
		return
	}
	s.sender.SendCallback(data.DownloadRequestResultCmd, requestResult, id, func() {
		// could not send data
		log.Println("Could not send download request result!")
		return
	})
}

func (s *ShareUploader) Fail(fileId string, chunkChecksum string, nodeId string) {
	s.deny(fileId, chunkChecksum, nodeId)
}

func (s *ShareUploader) uploadSuccess() {
	log.Println("Release upload token")
	s.uploadTokens <- 1
}

func (s *ShareUploader) uploadFail() {
	log.Println("Release upload token")
	s.uploadTokens <- 1
}

func (s *ShareUploader) sendData(sf *commonData.SharedFile, conn *net.TCPConn, fileId string, chunkChecksum string, downloadPath string) error {
	// get file path to finished file, or currently downloading file with download extension
	filePath, err := s.findPath(sf, downloadPath)
	if err != nil {
		return err
	}

	// open file
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return err
	}
	// send data
	chunk, ok := sf.ChunkById(chunkChecksum)
	if !ok || !chunk.IsLocal() {
		return errors.New("Cannot send data, requested chunk not available!")
	}
	sectionReader := io.NewSectionReader(file, chunk.Offset(), chunk.Size())
	io.Copy(conn, sectionReader)

	return nil
}

func (s *ShareUploader) findPath(sf *commonData.SharedFile, downloadPath string) (string, error) {
	// if file is local try to find file in 'shared' directories first
	found := false
	path := sf.FilePath()
	if sf.IsLocal() {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			found = true
		}
	}
	if found {
		return path, nil
	}
	// else search file in download directory
	path = downloadPath
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		found = true
	}
	if found {
		return path, nil
	}
	return path, errors.New("Cannot upload! File not found")
}
