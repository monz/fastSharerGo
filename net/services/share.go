package net

import (
	"errors"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	tools "github.com/monz/fastSharerGo/common/util"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
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
	sharedFiles       map[string]*commonData.SharedFile
	sender            Sender
	uploader          Uploader
	downloader        Downloader
	fileInfoer        FileInfoer
	stopped           bool
	mu                sync.Mutex
}

func NewShareService(localNodeId uuid.UUID, senderChan chan data.ShareCommand, infoPeriod time.Duration, downloadDir string, maxUploads int, maxDownloads int) *ShareService {
	s := new(ShareService)
	s.localNodeId = localNodeId
	s.nodes = make(map[uuid.UUID]*data.Node)
	s.infoPeriod = infoPeriod
	s.downloadDir = downloadDir
	s.downloadExtension = ".part" // todo: load from paramter
	s.maxUploads = util.InitSema(maxUploads)
	s.maxDownloads = util.InitSema(maxDownloads)
	s.sharedFiles = make(map[string]*commonData.SharedFile)
	s.sender = NewShareSender(senderChan)
	s.uploader = NewShareUploader(localNodeId, s.maxUploads, s.sender)
	s.downloader = NewShareDownloader(s.maxDownloads)
	s.fileInfoer = NewSharedFileInfoer(localNodeId, s.sender)
	s.stopped = false
	// init random
	rand.Seed(time.Now().UnixNano())

	return s
}

func (s *ShareService) Start() {
	// start shared file info sender
	go s.sendSharedFileInfo()
}

func (s *ShareService) Stop() {
	// stop sending shared file info messages
	s.stopped = true
}

func (s *ShareService) sendSharedFileInfo() {
	log.Println("Starting info service for shared files")
	for !s.stopped {
		s.mu.Lock()
		for _, sf := range s.sharedFiles {
			for _, node := range s.nodes {
				replicaNode, ok := sf.ReplicaNodeById(node.Id())
				if !ok || !replicaNode.IsAllInfoReceived() {
					log.Println("Send shared files info message")
					s.fileInfoer.SendFileInfo(*sf, node.Id())
				} else if len(sf.Checksum()) > 0 && !replicaNode.IsCompleteMsgSent() {
					log.Println("Send 'complete state message'")
					// only send complete message once, 'completeMsgSent' is internal state, does not get shared!
					replicaNode.SetCompleteMsgSent(true)
					s.fileInfoer.SendCompleteMsg(*sf, node.Id())
				}
			}
		}
		s.mu.Unlock()
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
		s.uploader.Deny(r)
		return
	}
	chunk, ok := sf.ChunkById(r.ChunkChecksum())
	if !ok || !chunk.IsLocal() {
		s.uploader.Deny(r)
		return
	}
	// acquire upload token
	log.Println("Waiting to acquire upload token...")
	select {
	case <-s.maxUploads:
		log.Println("Could acquire upload token")
		s.uploader.Accept(r, sf, s.downloadFilePath(sf, !sf.IsLocal()))
	case <-time.After(tokenAcquireTimeout):
		s.uploader.Deny(r)
		return
	}
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedDownloadRequestResult(requestResult data.DownloadRequestResult) {
	log.Println("Added answer to our download request")
	go s.download(requestResult)
}

func (s *ShareService) download(rr data.DownloadRequestResult) {
	// check if requested chunk is local
	sf, ok := s.sharedFiles[rr.FileId()]
	if !ok {
		log.Println("Could not find shared file")
		s.downloader.Fail(nil)
		return
	}
	chunk, ok := sf.ChunkById(rr.ChunkChecksum())
	if !ok {
		log.Println("Could not find chunk:", rr.ChunkChecksum())
		s.downloader.Fail(nil)
		return
	}
	// check if remote node is still in node list
	id, err := uuid.Parse(rr.NodeId())
	if err != nil {
		log.Println(err)
		s.downloader.Fail(nil)
		return
	}
	node, ok := s.nodes[id]
	if !ok {
		log.Println("Download failed, node not found")
		s.downloader.Fail(nil)
		return
	}
	s.downloader.Download(sf, chunk, node, rr.DownloadPort(), s.downloadFilePath(sf, true))
}

func (s *ShareService) downloadFilePath(sf *commonData.SharedFile, withExtension bool) string {
	filePath := filepath.Join(s.downloadDir, sf.FileRelativePath())
	if withExtension {
		filePath += s.downloadExtension
	}
	log.Println("THIS IS THE DOWNLOAD FILE PATH:", filePath)
	log.Println("THIS IS THE SHARED FILE PATH:", sf.FilePath())
	log.Println("THIS IS THE SHARED RELATIVE FILE PATH:", sf.FileRelativePath())
	return filePath
}

func (s *ShareService) helperPrint() {
	log.Printf("ShareSerive contains %d files\n", len(s.sharedFiles))
	//for key, _ := range s.sharedFiles {
	//	log.Printf("ShareService contains file '%s':\n", key)
	//}
}

// implement shareSubscriber interface
func (s *ShareService) ReceivedShareList(remoteSf commonData.SharedFile) {
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
		err := s.consolidateSharedFileInfo(sf, remoteSf)
		if err != nil {
			log.Println(err)
			return
		}
		//sf.ClearChunksWithoutChecksum() // unnecessary due to consolidation
		log.Println("First added shared file has chunkCount = ", len(sf.Chunks()))
		s.sharedFiles[sf.FileId()] = sf
	}

	filePath := s.downloadFilePath(sf, false)
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

func (s *ShareService) requestDownload(sf *commonData.SharedFile, initialDelay time.Duration) {
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
			s.downloader.Fail(nil)
			go s.requestDownload(sf, 500)
			return
		}

		// mark chunk as currently downloading
		if !chunk.ActivateDownload() {
			log.Println("Chunk is already downloading")
			s.downloader.Fail(nil)
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
			s.downloader.Fail(nil)
		})

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

func (s *ShareService) nextDownloadInformation(sf *commonData.SharedFile) (nodeId uuid.UUID, chunk *commonData.Chunk, err error) {
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

func (s *ShareService) consolidateSharedFileInfo(localSf *commonData.SharedFile, remoteSf commonData.SharedFile) error {
	log.Println("Consolidate shared file information")
	// clean paths
	localSf.SetFilePath(tools.CleanPath(remoteSf.FilePath()))
	localSf.SetFileRelativePath(tools.CleanPath(remoteSf.FileRelativePath()))
	// add replica nodes
	for _, node := range remoteSf.ReplicaNodes() {
		// skip localNode id
		if node.Id() == s.localNodeId {
			log.Println("Skipped local node in replica node list:", node.Id())
			continue
		}
		// skip unknown node
		_, ok := s.nodes[node.Id()]
		if !ok {
			log.Println("Skipped replica node:", node.Id())
			continue
		}
		// copy complete state
		//node.SetIsAllInfoReceived(node.IsAllInfoReceived()) // fix: this line does not make any sense
		localSf.AddReplicaNode(node)
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
		log.Println("In Consolidate, chunk state:", remoteChunk.IsLocal())
		localSf.AddChunk(remoteChunk)
	}
	return nil
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

func fileComplete(filePath string, checksum string, chunks []*commonData.Chunk) (bool, error) {
	checksums := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		checksum, err := util.CalculateChecksum(filePath, chunk.Offset(), chunk.Size())
		if err != nil {
			return false, err
		}
		checksums = append(checksums, checksum)
	}
	calculatedChecksum := util.SumOfChecksums(checksums)

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

// implement directoryChangeSubscriber interface
func (s *ShareService) AddLocalSharedFile(newSf commonData.SharedFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Println("Received info of local file:", newSf)
	// add to shared files
	if !s.isSharedFile(newSf) {
		s.sharedFiles[newSf.FileId()] = &newSf
	}
}

func (s *ShareService) isSharedFile(newSf commonData.SharedFile) bool {
	isShared := false

	for _, sf := range s.sharedFiles {
		if sf.FileId() == newSf.FileId() {
			isShared = true
			break
		}
	}
	return isShared
}
