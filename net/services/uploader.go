package net

import (
	"errors"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type Uploader interface {
	Accept(r data.DownloadRequest, sf *commonData.SharedFile, downloadPath string)
	Deny(r data.DownloadRequest)
}

type ShareUploader struct {
	localNodeId  uuid.UUID
	uploadTokens chan int
	sender       Sender
	mu           sync.Mutex
}

func NewShareUploader(localNodeId uuid.UUID, uploadTokens chan int, sender Sender) *ShareUploader {
	s := new(ShareUploader)
	s.localNodeId = localNodeId
	s.uploadTokens = uploadTokens
	s.sender = sender

	return s
}

func (s *ShareUploader) Accept(r data.DownloadRequest, sf *commonData.SharedFile, downloadPath string) {
	log.Println("Accept download request")
	// open random port
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// send port information to other client
	requestResult := []interface{}{data.NewDownloadRequestResult(r.FileId(), s.localNodeId.String(), r.ChunkChecksum(), l.Addr().(*net.TCPAddr).Port)}
	nodeId, err := uuid.Parse(r.NodeId())
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	s.sender.SendCallback(data.DownloadRequestResultCmd, requestResult, nodeId, func() {
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
	err = s.sendData(sf, conn, r.FileId(), r.ChunkChecksum(), downloadPath)
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// release upload token
	s.uploadSuccess()
}

func (s *ShareUploader) Deny(r data.DownloadRequest) {
	log.Println("Cannot accept download request")
	requestResult := []interface{}{data.NewDownloadRequestResult(r.FileId(), s.localNodeId.String(), r.ChunkChecksum(), data.DenyDownload)}
	nodeId, err := uuid.Parse(r.NodeId())
	if err != nil {
		log.Println(err)
		return
	}
	s.sender.SendCallback(data.DownloadRequestResultCmd, requestResult, nodeId, func() {
		// could not send data
		log.Println("Could not send download request result!")
		return
	})

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
