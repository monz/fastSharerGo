package net

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/google/uuid"
	localData "github.com/monz/fastSharerGo/data"
	"github.com/monz/fastSharerGo/net/data"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"path"
	"sync"
	"time"
)

const (
	// already defined in network service
	//socketTimeout = 10 * time.Second
	bufferSize = 4096
)

type ShareService struct {
	localNodeId  uuid.UUID
	nodes        map[uuid.UUID]*data.Node
	downloadDir  string
	maxUploads   chan int
	maxDownloads chan int
	sharedFiles  map[string]data.SharedFile
	sender       chan data.ShareCommand
	mu           sync.Mutex
}

func NewShareService(localNodeId uuid.UUID, sender chan data.ShareCommand, downloadDir string, maxUploads int, maxDownloads int) *ShareService {
	s := new(ShareService)
	s.localNodeId = localNodeId
	s.nodes = make(map[uuid.UUID]*data.Node)
	s.downloadDir = downloadDir
	s.maxUploads = initSema(maxUploads)
	s.maxDownloads = initSema(maxDownloads)
	s.sharedFiles = make(map[string]data.SharedFile)
	s.sender = sender
	// init random
	rand.Seed(time.Now().UnixNano())

	return s
}

func initSema(n int) chan int {
	channel := make(chan int, n)
	for i := 0; i < n; i++ {
		channel <- 1
	}
	return channel
}

func (s ShareService) Start() {
}

func (s ShareService) Stop() {
}

// implement shareSubscriber interface
func (s ShareService) ReceivedDownloadRequest() {
	log.Println("Added download request from other client")
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedDownloadRequestResult(requestResult data.DownloadRequestResult) {
	log.Println("Added answer to our download request")
	go s.download(requestResult)
}

func (s *ShareService) download(rr data.DownloadRequestResult) {
	sf, ok := s.sharedFiles[rr.FileId()]
	if !ok {
		log.Println("Could not find shared file")
		s.downloadFail(nil)
		return
	}
	chunk, err := sf.ChunkById(rr.ChunkChecksum())
	if err != nil {
		log.Println(err)
		s.downloadFail(nil)
		return
	}
	// check if download was accepted
	if rr.DownloadPort() < 0 {
		log.Printf("Download request of chunk '%s' was not accepted\n", rr.ChunkChecksum())
		s.downloadFail(chunk)
		return
	}

	log.Printf("Downloading for file '%s', chunk '%s'\n", rr.FileId(), rr.ChunkChecksum())
	// connect to remote node
	tcpConn, err := s.connectToRemote(rr.NodeId(), rr.DownloadPort())
	if err != nil {
		log.Println(err)
		s.downloadFail(chunk)
		return
	}
	defer tcpConn.Close()

	// download chunk
	checksum, err := s.receiveData(tcpConn, sf, chunk)
	if err != nil {
		log.Println(err)
		s.downloadFail(chunk)
	} else if checksum != rr.ChunkChecksum() {
		log.Println("Checksum of downloaded chunk does not match!")
		s.downloadFail(chunk)
	} else {
		s.downloadSuccess(sf, chunk)
	}
}

func (s *ShareService) connectToRemote(nodeId string, port int) (*net.TCPConn, error) {
	id, err := uuid.Parse(nodeId)
	if err != nil {
		return nil, err
	}
	node, ok := s.nodes[id]
	if !ok {
		return nil, errors.New("Download failed, node not found")
	}

	var tcpConn *net.TCPConn
	for _, ip := range node.Ips() {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf(ip, port), socketTimeout)
		tcpConn = conn.(*net.TCPConn)
		if err != nil {
			return nil, err
		}
		break
	}
	return tcpConn, nil
}

func (s *ShareService) downloadFail(chunk *localData.Chunk) {
	// todo: implement
}

func (s *ShareService) downloadSuccess(sf data.SharedFile, chunk *localData.Chunk) {
	// todo: implement
}

func (s *ShareService) receiveData(conn *net.TCPConn, sf data.SharedFile, chunk *localData.Chunk) (checksum string, err error) {
	// create directory structure including download directory
	path := path.Dir(path.Join(s.downloadDir, sf.FileRelativePath()))
	err = os.MkdirAll(path, os.ModeDir)
	if err != nil {
		return checksum, err
	}

	// write data to file
	file, err := os.Open(path)
	if err != nil {
		return checksum, err
	}

	// advance read pointer to offset
	if chunk.Offset() > 0 {
		_, err = file.Seek(chunk.Offset(), 0)
		if err != nil {
			return checksum, err
		}
	}

	// prepare message digest
	hash := md5.New()

	// simultaneously download, write to file, calculate checksum
	remainingBytes := chunk.Size()
	buf := make([]byte, bufferSize)
	bufReader := bytes.NewReader(buf)
	reader := bufio.NewReader(conn)
	n, err := reader.Read(buf)
	if err != nil {
		return checksum, err
	}
	multiWriter := io.MultiWriter(file, hash)
	for n > 0 && err != io.EOF {
		if (remainingBytes - int64(n)) > 0 {
			_, err = io.Copy(multiWriter, bufReader)
			if err != nil {
				return checksum, err
			}
			//file.Write(buf, 0, n)
			remainingBytes -= int64(n)
			n, err = reader.Read(buf)
		} else {
			_, err = io.CopyN(multiWriter, bufReader, int64(remainingBytes))
			//file.Write(buf, 0, remainingBytes)
			break
		}
	}
	file.Close()

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedShareList(remoteSf data.SharedFile) {
	log.Println("Added shared file from other client")

	sf, err := s.consolidateSharedFileInfo(remoteSf)
	if err != nil {
		log.Println(err)
		return
	}

	isExisting := fileExists(sf.FilePath())
	isComplete := fileComplete(sf.FilePath())
	if isExisting && isComplete {
		log.Println("File already downloaded")
		return
	} else if isExisting && !isComplete {
		log.Println("Delete corrupted file")
		err := os.Remove(sf.FilePath())
		if err != nil {
			log.Println(err)
			return
		}
	} else if sf.IsDownloadActive() {
		log.Printf("Download of file '%s' already active\n", sf.FilePath())
		return
	} else {
		// maybe move activateDownload into download function
		// currently used to mark sharedFile as 'already handled'
		// to prevent cyclicly checking if downloaded files are complete
		if ok := sf.ActivateDownload(); !ok {
			log.Fatal("Could not activate download of file", sf.FilePath())
		}
	}

	// check for enough space on disk
	if !enoughSpace(sf.FilePath()) {
		log.Println("Not enough disk space to download file:", sf.FilePath())
		return
	}

	// activate download
	go s.requestDownload(&sf, 0)
}

func (s *ShareService) requestDownload(sf *data.SharedFile, initialDelay time.Duration) {
	log.Printf("Request download for sharedFile %p\n", sf)
	// delay download request
	time.Sleep(initialDelay * time.Millisecond)

	// limit request to maxDownload count
	// take download token
	// todo: add timeout with switch;select...??
	<-s.maxDownloads
	defer func() { s.maxDownloads <- 1 }()
	log.Println("Could aquire download token")

	// get list of all chunks still to download
	chunkCount := len(sf.ChunksToDownload())
	for chunkCount > 0 {
		log.Printf("Remaining chunks to download: %d, for file %p\n", chunkCount, sf)
		time.Sleep(5 * time.Second)
		//select node to download from
		nodeId, chunk, err := s.nextDownloadInformation(sf)
		if err != nil {
			log.Println(err)
			// reschedule download job
			log.Println("Reschedule download job")
			go s.requestDownload(sf, 500)
			return
		}

		// mark chunk as currently downloading
		// todo: currently activate download does not work
		// maybe working on chunk copy not reference!!!!!
		if !chunk.ActivateDownload() {
			log.Println("Chunk is already downloading")
			continue
		}

		// send download request for chunk
		request := []interface{}{data.NewDownloadRequest(sf.FileId(), s.localNodeId.String(), chunk.Checksum())}
		cmd := data.NewShareCommand(data.DownloadRequestCmd, request, nodeId, func() {
			log.Println("Could not send message!")
			//go s.requestDownload(sf, 500)
			if !chunk.DeactivateDownload() {
				log.Println("Could not deactivate download of chunk", chunk.Checksum())
			}
		})
		s.sender <- *cmd

		// check wether new chunk information arrived
		chunkCount = len(sf.ChunksToDownload())
	}
	// no more download information, check whether file is completely downloaded
	if chunkCount <= 0 && !sf.IsLocal() {
		log.Println("No chunks to download, but files is not local yet, waiting for more information!")
		// reschedule download job
		log.Println("Reschedule download job")
		go s.requestDownload(sf, 1500) // todo: reduce time to 500ms
		return
	} else if chunkCount <= 0 && sf.IsLocal() {
		log.Printf("Download of file '%s' finished.\n", sf.FilePath())
		return
	}
}

func (s *ShareService) nextDownloadInformation(sf *data.SharedFile) (nodeId uuid.UUID, chunk *localData.Chunk, err error) {
	// get all replica nodes holding information about remaining chunks to download
	// select replica node which holds least shared information, to spread information
	// more quickly in entire network
	// todo: implement; currently next chunk is randomly chosen
	chunks := sf.ChunksToDownload()
	if len(chunks) <= 0 {
		return nodeId, chunk, errors.New("Currently no chunks to download")
	}

	chunkDist := make(map[string][]data.ReplicaNode)
	for _, c := range chunks {
		log.Printf("In nextDownload chunk %p\n", c)
		replicaNodes := sf.ReplicaNodesByChunk(c.Checksum())
		if len(replicaNodes) <= 0 {
			continue
		}
		_, ok := chunkDist[c.Checksum()]
		if !ok {
			chunkDist[c.Checksum()] = replicaNodes
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
		if c.Checksum() == chunkSum {
			chunk = c
			break
		}
	}
	return nodeId, chunk, nil
}

func (s *ShareService) consolidateSharedFileInfo(remoteSf data.SharedFile) (data.SharedFile, error) {
	// todo: implement
	return remoteSf, nil
}

func fileExists(filePath string) bool {
	// todo: implement
	return false
}

func fileComplete(filePath string) bool {
	// todo: implement
	return true
}

func enoughSpace(filePath string) bool {
	// todo: implement
	return true
}

// implement nodeSubscriber interface
func (s *ShareService) AddNode(newNode data.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.nodes[newNode.Id()]
	if !ok {
		s.nodes[newNode.Id()] = &newNode
	}
}

// implement nodeSubscriber interface
func (s *ShareService) RemoveNode(node data.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.nodes, node.Id())
}
