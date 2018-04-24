package net

import (
	"errors"
	"github.com/google/uuid"
	localData "github.com/monz/fastSharerGo/data"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

type ShareService struct {
	localNodeId  uuid.UUID
	maxUploads   chan int
	maxDownloads chan int
	sharedFiles  []data.SharedFile
	sender       chan data.ShareCommand
	mu           sync.Mutex
}

func NewShareService(localNodeId uuid.UUID, sender chan data.ShareCommand, maxUploads int, maxDownloads int) *ShareService {
	s := new(ShareService)
	s.localNodeId = localNodeId
	s.maxUploads = initSema(maxUploads)
	s.maxDownloads = initSema(maxDownloads)
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

func (s ShareService) ReceivedDownloadRequest() {
	log.Println("Added download request from other client")
}

func (s ShareService) ReceivedDownloadRequestResult() {
	log.Println("Added answer to our download request")
}

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
