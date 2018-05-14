package net

import (
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	tools "github.com/monz/fastSharerGo/common/util"
	"github.com/monz/fastSharerGo/net/data"
	"log"
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
	sharedFiles       map[string]*commonData.SharedFile
	sender            Sender
	uploader          Uploader
	downloader        Downloader
	fileInfoer        FileInfoer
	stopped           bool
	mu                sync.Mutex
}

func NewShareService(localNodeId uuid.UUID, sender Sender, uploader Uploader, downloader Downloader, infoPeriod time.Duration, downloadDir string) *ShareService {
	s := new(ShareService)
	s.localNodeId = localNodeId
	s.nodes = make(map[uuid.UUID]*data.Node)
	s.infoPeriod = infoPeriod
	s.downloadDir = downloadDir
	s.downloadExtension = ".part" // todo: load from paramter
	s.sharedFiles = make(map[string]*commonData.SharedFile)
	s.sender = sender
	s.uploader = uploader
	s.downloader = downloader
	s.fileInfoer = NewSharedFileInfoer(localNodeId, s.sender)
	s.stopped = false

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
		s.uploader.Fail(r.FileId(), r.ChunkChecksum(), r.NodeId())
		return
	}
	chunk, ok := sf.ChunkById(r.ChunkChecksum())
	if !ok || !chunk.IsLocal() {
		s.uploader.Fail(r.FileId(), r.ChunkChecksum(), r.NodeId())
		return
	}
	// start upload
	s.uploader.Upload(sf, chunk.Checksum(), r.NodeId(), s.downloadFilePath(sf, !sf.IsLocal()))
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
		s.downloader.Fail()
		return
	}
	chunk, ok := sf.ChunkById(rr.ChunkChecksum())
	if !ok {
		log.Println("Could not find chunk:", rr.ChunkChecksum())
		s.downloader.Fail()
		return
	}
	// check if remote node is still in node list
	id, err := uuid.Parse(rr.NodeId())
	if err != nil {
		log.Println(err)
		s.downloader.Fail()
		return
	}
	node, ok := s.nodes[id]
	if !ok {
		log.Println("Download failed, node not found")
		s.downloader.Fail()
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

// implement shareSubscriber interface
func (s *ShareService) ReceivedShareList(remoteSf commonData.SharedFile) {
	log.Println("Added shared file from other client")
	go s.requestDownload(remoteSf)
}

func (s *ShareService) requestDownload(remoteSf commonData.SharedFile) {
	log.Printf("ShareSerive contains %d files\n", len(s.sharedFiles))
	// add/update share file list, consolidate shared file information
	// always consolidate information, to get all information about files in network
	sf, err := s.updateSharedFileList(&remoteSf)
	if err != nil {
		log.Println(err)
		return
	}
	// if file is shared, check if it is local, if is local, stop here
	if sf.IsLocal() {
		log.Println("File is local, do not have to download")
		return
	}
	// request/activate download
	// todo: think about maxDownload/maxUpload sema handling
	// where to take the tokens, who holds the tokens...
	filePath := s.downloadFilePath(sf, false)
	s.downloader.RequestDownload(sf, filePath)
}

func (s *ShareService) updateSharedFileList(remoteSf *commonData.SharedFile) (*commonData.SharedFile, error) {
	sf, ok := s.sharedFiles[remoteSf.FileId()]
	if ok {
		// update
		log.Printf("Update shared file information of file '%s'\n", sf.FileName())
		if err := s.consolidateSharedFileInfo(sf, remoteSf); err != nil {
			return nil, err
		}
	} else {
		// add
		log.Printf("First added shared file information of file '%s'\n", remoteSf.FileName())
		sf = remoteSf
		sf.ClearReplicaNodes() // fix: needs another solution, deletes replica nodes in remoteSf, too, due to reference
		if err := s.consolidateSharedFileInfo(sf, remoteSf); err != nil {
			return nil, err
		}
		s.sharedFiles[sf.FileId()] = sf
	}
	return sf, nil
}

func (s *ShareService) consolidateSharedFileInfo(localSf *commonData.SharedFile, remoteSf *commonData.SharedFile) error {
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
		localSf.UpdateReplicaNode(node)
	}
	// update shared file checksum
	if len(localSf.Checksum()) <= 0 {
		localSf.SetChecksum(remoteSf.Checksum())
	}
	// add new chunk information
	localSf.ClearChunksWithoutChecksum()
	for _, remoteChunk := range remoteSf.Chunks() {
		if len(remoteChunk.Checksum()) <= 0 {
			continue
		}
		log.Println("In Consolidate, chunk state:", remoteChunk.IsLocal())
		localSf.AddChunk(remoteChunk)
	}
	return nil
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
	_, isShared := s.sharedFiles[newSf.FileId()]
	if !isShared {
		s.sharedFiles[newSf.FileId()] = &newSf
	}
}
