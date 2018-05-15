package net

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	downloadRescheduleDelay = 5000 // milliseconds
)

type Downloader interface {
	RequestDownload(sf *commonData.SharedFile, filePath string)
	Download(sf *commonData.SharedFile, chunk *commonData.Chunk, node *data.Node, downloadPort int, downloadPath string)
	Fail()
}

type ShareDownloader struct {
	localNodeId    uuid.UUID
	downloadTokens chan int
	sender         Sender
	mu             sync.Mutex
}

func NewShareDownloader(localNodeId uuid.UUID, downloadTokens int, sender Sender) *ShareDownloader {
	s := new(ShareDownloader)
	s.localNodeId = localNodeId
	s.downloadTokens = util.InitSema(downloadTokens)
	s.sender = sender
	// init random
	rand.Seed(time.Now().UnixNano())

	return s
}

func (s *ShareDownloader) RequestDownload(sf *commonData.SharedFile, filePath string) {
	// if download already active nothing to do
	if sf.IsDownloadActive() || sf.IsValidationActive() {
		log.Printf("Download of file '%s' already active\n", sf.FileName())
		return
	}
	// check if file already exists
	isExisting := fileExists(filePath)
	if isExisting {
		// check whether file is valid
		isValid, err := s.validateFile(sf, filePath)
		if err != nil {
			log.Println(err)
			return
		}
		if isValid {
			log.Println("File was already downloaded, skipping...", sf.FileName())
			return
		} else {
			log.Println("Delete corrupted file, subsequently start download")
			if err := os.Remove(filePath); err != nil {
				log.Println(err)
				return
			}
		}
	}
	// check for enough space on disk
	if !enoughSpace(sf.FilePath()) {
		log.Println("Not enough disk space to download file:", sf.FileName())
		return
	}
	// activate download of shared file
	log.Println("Activate download of file:", sf.FileName())
	if ok := sf.ActivateDownload(); !ok {
		log.Printf("Could not activate download of file '%s'\n", sf.FileName())
		return
	}
	// schedule download
	s.scheduleDownload(sf, 0)
}

func (s *ShareDownloader) validateFile(sf *commonData.SharedFile, filePath string) (bool, error) {
	log.Println("Check if existing file is valid")

	// start validation
	if ok := sf.ActivateValidation(); !ok {
		return false, errors.New(fmt.Sprintf("Could not activate validation state of file '%s'\n", sf.FileName()))
	}
	// finish validation on function return
	defer func() {
		if ok := sf.DeactivateValidation(); !ok {
			log.Println(fmt.Sprintf("Could not deactivate validation state of file '%s'\n", sf.FileName()))
		}
	}()
	// check if file checksum information arrived, yet
	if len(sf.Checksum()) <= 0 {
		return false, errors.New(fmt.Sprintf("Coult not check checksum of file '%s', waiting for checksum\n", sf.FileName()))
	}
	// calculate and compare all file checksums
	isValid := util.CompareFileChecksums(sf.FileId(), filePath, sf.FileSize(), sf.Checksum())
	if isValid {
		sf.SetAllChunksLocal(true)
	}
	log.Printf("State of file: existing = '%t', isValid = '%t'\n", true, isValid)

	return isValid, nil
}

func fileExists(filePath string) bool {
	var exists bool
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exists = false
	} else {
		exists = true
	}
	return exists
}

func enoughSpace(filePath string) bool {
	// todo: implement
	return true
}

func (s *ShareDownloader) scheduleDownload(sf *commonData.SharedFile, initialDelay time.Duration) {
	log.Printf("Request download for sharedFile %p\n", sf)

	// delay download request
	time.Sleep(initialDelay * time.Millisecond)

	// get list of all chunks still to download
	chunkCount := len(sf.ChunksToDownload())
	for chunkCount > 0 {
		log.Printf("Remaining chunks to download: %d, for file %p\n", chunkCount, sf)
		log.Println("Waiting while acquiring download token...")
		// limit request to maxDownload count
		// take download token
		<-s.downloadTokens
		log.Println("Could aquire download token")

		//select node to download from
		nodeId, chunk, err := s.nextDownloadInformation(sf)
		if err != nil {
			log.Println(err)
			// reschedule download job
			log.Println("Reschedule download job")
			s.downloadFail(nil)
			go s.scheduleDownload(sf, downloadRescheduleDelay)
			return
		}

		// mark chunk as currently downloading
		if !chunk.ActivateDownload() {
			log.Println("Chunk is already downloading")
			s.downloadFail(nil)
			continue
		}

		// send download request for chunk
		request := []interface{}{data.NewDownloadRequest(sf.FileId(), s.localNodeId.String(), chunk.Checksum())}
		s.sender.SendCallback(data.DownloadRequestCmd, request, nodeId, func() {
			log.Println("Could not send message!")
			if !chunk.DeactivateDownload() {
				log.Println("Could not deactivate download of chunk", chunk.Checksum())
			}
			// release download token
			s.downloadFail(nil)
		})

		// check whether new chunk information arrived
		chunkCount = len(sf.ChunksToDownload())
	}
	// no more download information, check whether file is completely downloaded
	isFileLocal := sf.IsLocal()
	if chunkCount <= 0 && !isFileLocal {
		log.Println("No chunks to download, but file is not local yet, waiting for more information!")
		// reschedule download job
		log.Println("Reschedule download job")
		go s.scheduleDownload(sf, downloadRescheduleDelay)
		return
	} else if chunkCount <= 0 && isFileLocal {
		log.Printf("Download of file '%s' finished.\n", sf.FileName())
		return
	} else {
		log.Fatal("Should never be reached!")
	}
}

func (s *ShareDownloader) nextDownloadInformation(sf *commonData.SharedFile) (nodeId uuid.UUID, chunk *commonData.Chunk, err error) {
	// get all replica nodes holding information about remaining chunks to download
	// select replica node which holds least shared information, to spread information
	// more quickly in entire network
	// todo: implement; currently next chunk is randomly chosen
	chunks := sf.ChunksToDownload()
	if len(chunks) <= 0 {
		return nodeId, chunk, errors.New("Currently no chunks to download")
	}

	chunkDist := make(map[string][]commonData.ReplicaNode)
	for _, c := range chunks {
		replicaNodes := sf.ReplicaNodesByChunk(c)
		if len(replicaNodes) <= 0 {
			continue
		}
		_, ok := chunkDist[c]
		if !ok {
			chunkDist[c] = replicaNodes
		}
	}
	if len(chunkDist) <= 0 {
		return nodeId, chunk, errors.New("Currently no replica nodes available")
	}

	minCount := math.MaxInt32
	var chunkSum string
	for chunkChecksum, replicaNodes := range chunkDist {
		if len(replicaNodes) < minCount {
			minCount = len(replicaNodes)
			log.Println("Number of replica nodes:", minCount)
			chunkSum = chunkChecksum
			nodeId = replicaNodes[rand.Intn(len(replicaNodes))].Id()
		}
	}

	// refactor 'nextDownloadInformation' function because of the following
	// have to search for chunk object, there might be a better solution
	for _, c := range chunks {
		if c == chunkSum {
			var ok bool
			chunk, ok = sf.ChunkById(c) // cannot use ':=' outer 'chunk' maybe gets shadowed
			if !ok {
				return nodeId, chunk, errors.New("Chunk not found")
			}
			break
		}
	}
	return nodeId, chunk, nil
}

func (s *ShareDownloader) Download(sf *commonData.SharedFile, chunk *commonData.Chunk, node *data.Node, downloadPort int, downloadPath string) {
	// check if download was accepted
	if downloadPort < 0 {
		log.Printf("Download request of chunk '%s' was not accepted\n", chunk.Checksum())
		s.downloadFail(chunk)
		return
	}

	log.Printf("Downloading for file '%s', chunk '%s'\n", sf.FileId(), chunk.Checksum())
	// connect to remote node
	tcpConn, err := s.connectToRemote(*node, downloadPort)
	if err != nil {
		log.Println(err)
		s.downloadFail(chunk)
		return
	}
	defer tcpConn.Close()

	// download chunk
	checksum, filePath, err := s.receiveData(tcpConn, sf, chunk, downloadPath)
	if err != nil {
		log.Println(err)
		s.downloadFail(chunk)
	} else if checksum != chunk.Checksum() {
		log.Printf("Checksum of downloaded chunk does not match! Was '%s', expected '%s'\n", checksum, chunk.Checksum())
		s.downloadFail(chunk)
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

func (s *ShareDownloader) downloadSuccess(sf *commonData.SharedFile, chunk *commonData.Chunk, oldFilePath string) {
	// need lock for renaming file only once
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *ShareDownloader) downloadFail(chunk *commonData.Chunk) {
	if chunk != nil {
		log.Printf("Download of chunk '%s' failed.", chunk.Checksum())
		chunk.DeactivateDownload()
	}
	// release download token
	s.downloadTokens <- 1
	log.Println("Released download token")
}

func (s *ShareDownloader) Fail() {
	s.downloadFail(nil)
}
