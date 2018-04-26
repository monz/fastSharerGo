package net

import (
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
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// already defined in network service
	tokenAcquireTimeout = 5 * time.Second
	bufferSize          = 4096
)

type ShareService struct {
	localNodeId       uuid.UUID
	nodes             map[uuid.UUID]*data.Node
	infoPeriod        time.Duration
	downloadDir       string
	downloadExtension string
	maxUploads        chan int
	maxDownloads      chan int
	sharedFiles       map[string]*data.SharedFile
	sender            chan data.ShareCommand
	dirSplit          *regexp.Regexp
	mu                sync.Mutex
}

func NewShareService(localNodeId uuid.UUID, sender chan data.ShareCommand, infoPeriod time.Duration, downloadDir string, maxUploads int, maxDownloads int) *ShareService {
	s := new(ShareService)
	s.localNodeId = localNodeId
	s.nodes = make(map[uuid.UUID]*data.Node)
	s.infoPeriod = infoPeriod
	s.downloadDir = downloadDir
	s.downloadExtension = ".part" // load from paramter
	s.maxUploads = initSema(maxUploads)
	s.maxDownloads = initSema(maxDownloads)
	s.sharedFiles = make(map[string]*data.SharedFile)
	s.sender = sender
	s.dirSplit = regexp.MustCompile("[/\\\\]")
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

func (s *ShareService) Start() {
	// start shared file info sender
	go s.sendSharedFileInfo()
}

func (s *ShareService) Stop() {
	// todo: implement
}

func (s *ShareService) sendSharedFileInfo() {
	log.Println("Starting info service for shared files")
	for {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, sf := range s.sharedFiles {
			for _, node := range s.nodes {
				replicaNode, ok := sf.ReplicaNodeById(node.Id())
				if !ok || !replicaNode.IsComplete() {
					// send all information
					// add local node as replica node for all local chunks
					log.Println("Send shared files info message")
					chunkSums := sf.LocalChunksChecksums()
					sfCopy := *sf
					localNode := data.NewReplicaNode(s.localNodeId, chunkSums, sf.IsLocal())
					sfCopy.AddReplicaNode(*localNode)

					// send data
					shareList := []interface{}{sfCopy}
					cmd := data.NewShareCommand(data.PushShareListCmd, shareList, node.Id(), func() {
						// could not send data
						log.Println("Could not send share list data")
						return
					})
					s.sender <- *cmd
				} else {
					// send 'complete state' message
					log.Println("Send 'complete state message'")
					// only send complete message once
					replicaNode.SetStopSharedInfo(true)
					// send information that node is complete
					sfCopy := *sf
					localNode := data.NewReplicaNode(s.localNodeId, []string{}, len(sf.Checksum()) > 0)
					sfCopy.AddReplicaNode(*localNode)

					// send data
					shareList := []interface{}{sfCopy}
					cmd := data.NewShareCommand(data.PushShareListCmd, shareList, node.Id(), func() {
						// could not send data
						log.Println("Could not send complete message")
						return
					})
					s.sender <- *cmd
				}
			}
		}
		// wait
		time.Sleep(s.infoPeriod)
	}
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedDownloadRequest(request data.DownloadRequest) {
	log.Println("Added download request from other client")
	go s.upload(request)
}

func (s *ShareService) upload(r data.DownloadRequest) {
	// check if requested chunk is local
	sf, ok := s.sharedFiles[r.FileId()]
	if !ok {
		s.denyUpload(r)
		return
	}
	chunk, ok := sf.ChunkById(r.ChunkChecksum())
	if !ok || !chunk.IsLocal() {
		s.denyUpload(r)
		return
	}
	// acquire upload token
	log.Println("Waiting to acquire upload token...")
	select {
	case <-s.maxUploads:
		log.Println("Could acquire upload token")
		s.acceptUpload(r)
	case <-time.After(tokenAcquireTimeout):
		s.denyUpload(r)
		return
	}
}

func (s *ShareService) acceptUpload(r data.DownloadRequest) {
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
	cmd := data.NewShareCommand(data.DownloadRequestResultCmd, requestResult, nodeId, func() {
		// could not send data
		log.Println("Could not send download request result!")
		s.uploadFail()
		return
	})
	s.sender <- *cmd
	// wait for other client to connect and upload chunk data
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
	err = s.sendData(conn, r.FileId(), r.ChunkChecksum())
	if err != nil {
		log.Println(err)
		s.uploadFail()
		return
	}
	// release upload token
	s.uploadSuccess()
}

func (s *ShareService) denyUpload(r data.DownloadRequest) {
	log.Println("Cannot accept download request")
	requestResult := []interface{}{data.NewDownloadRequestResult(r.FileId(), s.localNodeId.String(), r.ChunkChecksum(), data.DenyDownload)}
	nodeId, err := uuid.Parse(r.NodeId())
	if err != nil {
		log.Println(err)
		return
	}
	cmd := data.NewShareCommand(data.DownloadRequestResultCmd, requestResult, nodeId, func() {
		// could not send data
		log.Println("Could not send download request result!")
		return
	})
	s.sender <- *cmd
}

func (s *ShareService) sendData(conn *net.TCPConn, fileId string, chunkChecksum string) error {
	sf, ok := s.sharedFiles[fileId]
	if !ok {
		return errors.New("Cannot send data, file not shared!")
	}
	chunk, ok := sf.ChunkById(chunkChecksum)
	if !ok || !chunk.IsLocal() {
		return errors.New("Cannot send data, requested chunk not available!")
	}

	// get file path to finished file, or currently downloading file with download extension
	filePath := s.downloadFilePath(*sf, !sf.IsLocal())

	// open file
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return err
	}
	// send data
	sectionReader := io.NewSectionReader(file, chunk.Offset(), chunk.Size())
	io.Copy(conn, sectionReader)

	return nil
}

func (s *ShareService) uploadSuccess() {
	log.Println("Release upload token")
	s.maxUploads <- 1
}

func (s *ShareService) uploadFail() {
	log.Println("Release upload token")
	s.maxUploads <- 1
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
	chunk, ok := sf.ChunkById(rr.ChunkChecksum())
	if !ok {
		log.Println("Could not find chunk:", rr.ChunkChecksum())
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
	checksum, filePath, err := s.receiveData(tcpConn, *sf, chunk)
	if err != nil {
		log.Println(err)
		s.downloadFail(chunk)
	} else if checksum != rr.ChunkChecksum() {
		log.Printf("Checksum of downloaded chunk does not match! Was '%s', expected '%s'\n", checksum, rr.ChunkChecksum())
		s.downloadFail(chunk)
	} else {
		s.downloadSuccess(sf, chunk, filePath)
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
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), socketTimeout)
		if err != nil {
			return nil, err
		}
		tcpConn = conn.(*net.TCPConn)
		break
	}
	return tcpConn, nil
}

func (s *ShareService) downloadFail(chunk *localData.Chunk) {
	if chunk != nil {
		log.Printf("Download of chunk '%s' failed.", chunk.Checksum())
		chunk.DeactivateDownload()
	}
	// release download token
	s.maxDownloads <- 1
	log.Println("Released download token")
}

func (s *ShareService) downloadSuccess(sf *data.SharedFile, chunk *localData.Chunk, oldFilePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("Download of chunk '%s' of file '%s' was successful\n", chunk.Checksum(), sf.FileId())
	chunk.SetLocal(true)
	chunk.DeactivateDownload()
	// check whether file was completley downloaded
	if sf.IsLocal() {
		log.Printf("Rename file '%s' to finish download\n", sf.FileName())
		err := os.Rename(oldFilePath, path.Join(path.Dir(oldFilePath), sf.FileName()))
		if err != nil {
			log.Println(err)
		}
		sf.DeactivateDownload()
	} else {
		log.Printf("File '%s' is not finished yet, chunk to download %d\n", sf.FileName(), len(sf.ChunksToDownload()))
	}
	// release download token
	s.maxDownloads <- 1
	log.Println("Released download token")
}

func (s *ShareService) receiveData(conn *net.TCPConn, sf data.SharedFile, chunk *localData.Chunk) (checksum string, filePath string, err error) {
	// create directory structure including download directory
	filePath = s.downloadFilePath(sf, true)
	log.Println("The download file path = ", filePath)
	downloadDir := path.Dir(filePath)
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
	hash := md5.New()

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

func (s *ShareService) helperPrint() {
	log.Printf("ShareSerive contains %d files\n", len(s.sharedFiles))
	//for key, _ := range s.sharedFiles {
	//	log.Printf("ShareService contains file '%s':\n", key)
	//}
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedShareList(remoteSf data.SharedFile) {
	log.Println("Added shared file from other client")
	s.helperPrint()
	// add/update share file list
	sf, ok := s.sharedFiles[remoteSf.FileId()]
	if ok {
		// if file is local stop here
		if sf.IsLocal() {
			log.Println("File is local, do not have to download")
			return
		}
		// update
		err := s.consolidateSharedFileInfo(sf, remoteSf)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		// add
		sf = &remoteSf
		sf.ClearReplicaNodes()
		for _, node := range remoteSf.ReplicaNodes() {
			// skip localNode id
			if node.Id() == s.localNodeId {
				continue
			}
			// skip unknown node
			_, ok := s.nodes[node.Id()]
			if !ok {
				continue
			}
			sf.AddReplicaNode(*node)
		}
		sf.ClearChunksWithoutChecksum()
		log.Println("First added shared file has chunkCount = ", len(sf.Chunks()))
		s.sharedFiles[sf.FileId()] = sf
	}

	filePath := s.downloadFilePath(*sf, false)
	log.Println("This is the file path:", filePath)
	isExisting := fileExists(filePath)
	isComplete := !isExisting
	if isExisting {
		var err error
		isComplete, err = fileComplete(filePath, sf.Checksum(), sf.Chunks())
		if err != nil {
			log.Println(err)
			return
		}
		if isComplete {
			sf.SetAllChunksLocal(true)
		}
	}
	log.Printf("State of file: existing = '%t', isComplete = '%t'\n", isExisting, isComplete)
	if isExisting && isComplete {
		log.Println("File already downloaded")
		return
	} else if isExisting && !isComplete {
		log.Println("Delete corrupted file")
		err := os.Remove(filePath)
		if err != nil {
			log.Println(err)
			return
		}
	} else if sf.IsDownloadActive() {
		log.Printf("Download of file '%s' already active\n", sf.FileName())
		return
	} else {
		// maybe move activateDownload into download function
		// currently used to mark sharedFile as 'already handled'
		// to prevent cyclicly checking if downloaded files are complete
		if ok := sf.ActivateDownload(); !ok {
			log.Fatal("Could not activate download of file", sf.FileName())
		}
	}

	// check for enough space on disk
	if !enoughSpace(sf.FilePath()) {
		log.Println("Not enough disk space to download file:", sf.FileName())
		return
	}

	// activate download
	go s.requestDownload(sf, 0)
}

func (s *ShareService) requestDownload(sf *data.SharedFile, initialDelay time.Duration) {
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
		<-s.maxDownloads
		//defer func() { s.maxDownloads <- 1; log.Println("Released download token") }()
		log.Println("Could aquire download token")

		//select node to download from
		nodeId, chunk, err := s.nextDownloadInformation(sf)
		if err != nil {
			log.Println(err)
			// reschedule download job
			log.Println("Reschedule download job")
			s.downloadFail(nil)
			go s.requestDownload(sf, 500)
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
		cmd := data.NewShareCommand(data.DownloadRequestCmd, request, nodeId, func() {
			log.Println("Could not send message!")
			if !chunk.DeactivateDownload() {
				log.Println("Could not deactivate download of chunk", chunk.Checksum())
			}
			// releas download token
			s.downloadFail(nil)
		})
		s.sender <- *cmd

		// check whether new chunk information arrived
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
		log.Printf("Download of file '%s' finished.\n", sf.FileName())
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

func (s *ShareService) consolidateSharedFileInfo(localSf *data.SharedFile, remoteSf data.SharedFile) error {
	// add replica nodes
	for _, node := range remoteSf.ReplicaNodes() {
		// skip localNode id
		if node.Id() == s.localNodeId {
			continue
		}
		// skip unknown node
		_, ok := s.nodes[node.Id()]
		if !ok {
			continue
		}
		localSf.AddReplicaNode(*node)
	}
	// update shared file checksum
	if len(localSf.Checksum()) <= 0 {
		localSf.SetChecksum(remoteSf.Checksum())
	}
	// add new chunk information
	for _, remoteChunk := range remoteSf.Chunks() {
		if len(remoteChunk.Checksum()) <= 0 {
			continue
		}
		localSf.AddChunk(remoteChunk)
	}
	return nil
}

func (s *ShareService) downloadFilePath(sf data.SharedFile, withExtension bool) string {
	filePath := path.Join(s.downloadDir, strings.Join(s.dirSplit.Split(sf.FileRelativePath(), -1), string(os.PathSeparator)))
	if withExtension {
		filePath += s.downloadExtension
	}
	log.Println("THIS IS THE FILE PATH:", filePath)
	return filePath
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

func fileComplete(filePath string, checksum string, chunks []*localData.Chunk) (bool, error) {
	// open file
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	completeHash := md5.New()
	for _, chunk := range chunks {
		// jump to offset
		sectionReader := io.NewSectionReader(file, chunk.Offset(), chunk.Size())
		// calculate checksum
		hash := md5.New()
		_, err = io.Copy(hash, sectionReader)
		if err != nil {
			return false, err
		}

		io.WriteString(completeHash, fmt.Sprintf("%x", hash.Sum(nil)))
	}
	calculatedChecksum := fmt.Sprintf("%x", completeHash.Sum(nil))

	log.Printf("FileComplete: expectedSum = %s, actualSum = %s\n", checksum, calculatedChecksum)
	return checksum == calculatedChecksum, nil
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
