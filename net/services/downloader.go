package net

import (
	"fmt"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

type Downloader interface {
	Download(sf *commonData.SharedFile, chunk *commonData.Chunk, node *data.Node, downloadPort int, downloadPath string)
	Fail(chunk *commonData.Chunk)
}

type ShareDownloader struct {
	downloadTokens chan int
}

func NewShareDownloader(downloadTokens chan int) *ShareDownloader {
	s := new(ShareDownloader)
	s.downloadTokens = downloadTokens

	return s
}

func (s *ShareDownloader) Download(sf *commonData.SharedFile, chunk *commonData.Chunk, node *data.Node, downloadPort int, downloadPath string) {
	// check if download was accepted
	if downloadPort < 0 {
		log.Printf("Download request of chunk '%s' was not accepted\n", chunk.Checksum())
		s.Fail(chunk)
		return
	}

	log.Printf("Downloading for file '%s', chunk '%s'\n", sf.FileId(), chunk.Checksum())
	// connect to remote node
	tcpConn, err := s.connectToRemote(*node, downloadPort)
	if err != nil {
		log.Println(err)
		s.Fail(chunk)
		return
	}
	defer tcpConn.Close()

	// download chunk
	checksum, filePath, err := s.receiveData(tcpConn, sf, chunk, downloadPath)
	if err != nil {
		log.Println(err)
		s.Fail(chunk)
	} else if checksum != chunk.Checksum() {
		log.Printf("Checksum of downloaded chunk does not match! Was '%s', expected '%s'\n", checksum, chunk.Checksum())
		s.Fail(chunk)
	} else {
		s.downloadSuccess(sf, chunk, filePath)
	}
}

func (s *ShareDownloader) connectToRemote(node data.Node, port int) (*net.TCPConn, error) {
	var tcpConn *net.TCPConn
	for _, ip := range node.Ips() {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), socketTimeout)
		if err != nil {
			return nil, err
		}
		tcpConn = conn.(*net.TCPConn)
		break
	}
	return tcpConn, nil
}

func (s *ShareDownloader) receiveData(conn *net.TCPConn, sf *commonData.SharedFile, chunk *commonData.Chunk, downloadPath string) (checksum string, filePath string, err error) {
	// create directory structure including download directory
	filePath = downloadPath
	log.Println("The download file path = ", filePath)
	downloadDir := filepath.Dir(filePath)
	log.Println("The download directory = ", downloadDir)
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		return checksum, filePath, err
	}

	// write data to file
	var file *os.File
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Println("File does not exist. Create new file.")
		file, err = os.Create(filePath)
		if err != nil {
			return checksum, filePath, err
		}
	} else {
		log.Println("File exists. Open file for write.")
		file, err = os.OpenFile(filePath, os.O_WRONLY, 0644)
		if err != nil {
			return checksum, filePath, err
		}
	}

	// advance read pointer to offset
	if chunk.Offset() > 0 {
		_, err = file.Seek(chunk.Offset(), 0)
		if err != nil {
			return checksum, filePath, err
		}
	}

	// prepare message digest
	hash := util.NewHash()

	// simultaneously download, write to file, calculate checksum
	multiWriter := io.MultiWriter(file, hash)
	n, err := io.Copy(multiWriter, conn)
	log.Printf("Could read '%d' bytes with err '%s'\n", n, err)
	if err != nil || n != chunk.Size() {
		return checksum, filePath, err
	}
	file.Close()

	return fmt.Sprintf("%x", hash.Sum(nil)), filePath, nil
}

func (s *ShareDownloader) Fail(chunk *commonData.Chunk) {
	if chunk != nil {
		log.Printf("Download of chunk '%s' failed.", chunk.Checksum())
		chunk.DeactivateDownload()
	}
	// release download token
	s.downloadTokens <- 1
	log.Println("Released download token")
}

func (s *ShareDownloader) downloadSuccess(sf *commonData.SharedFile, chunk *commonData.Chunk, oldFilePath string) {
	log.Printf("Download of chunk '%s' of file '%s' was successful\n", chunk.Checksum(), sf.FileId())
	chunk.SetLocal(true)
	chunk.DeactivateDownload()
	// check whether file was completley downloaded
	if sf.IsLocal() {
		log.Printf("Rename file '%s' to finish download\n", sf.FileName())
		err := os.Rename(oldFilePath, filepath.Join(filepath.Dir(oldFilePath), sf.FileName()))
		if err != nil {
			log.Println(err)
		}
		sf.DeactivateDownload()
	} else {
		log.Printf("File '%s' is not finished yet, chunk to download %d\n", sf.FileName(), len(sf.ChunksToDownload()))
	}
	// release download token
	s.downloadTokens <- 1
	log.Println("Released download token")
}
