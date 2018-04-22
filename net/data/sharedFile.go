package data

import (
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/data"
	"sync"
)

type SharedFile struct {
	FileMetadata     data.FileMetadata         `json:"metadata"`
	FileReplicaNodes map[uuid.UUID]ReplicaNode `json:"replicaNodes"`
	downloadActive   bool
	mu               sync.Mutex
}

func NewSharedFile(metadata data.FileMetadata) *SharedFile {
	sf := new(SharedFile)
	sf.FileMetadata = metadata

	return sf
}

func (sf SharedFile) FilePath() string {
	return sf.FileMetadata.Path()
}

func (sf SharedFile) SetFilePath(filePath string) {
	sf.FileMetadata.SetFilePath(filePath)
}

func (sf SharedFile) Metadata() data.FileMetadata {
	return sf.FileMetadata
}

func (sf SharedFile) Checksum() string {
	return sf.FileMetadata.Checksum()
}

func (sf SharedFile) IsLocal() bool {
	var isLocal bool
	expectedChunkCount := data.GetChunkCount(sf.FileMetadata.Size())
	actualChunkCount := len(sf.FileMetadata.Chunks())
	if actualChunkCount > 0 && actualChunkCount == expectedChunkCount {
		isLocal = sf.FileMetadata.AllChunksLocal()
	} else {
		isLocal = false
	}
	return isLocal
}

func (sf SharedFile) FileId() string {
	return sf.FileMetadata.Id()
}

func (sf SharedFile) ReplicaNodes() map[uuid.UUID]ReplicaNode {
	return sf.FileReplicaNodes
}

// use semaphores to make method atomic!!!
func (sf *SharedFile) activateDownload() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	var success bool
	if sf.downloadActive {
		success = false
	} else {
		sf.downloadActive = true
		success = true
	}
	return success
}

// use semaphores to make method atomic!!!
func (sf *SharedFile) deactivateDownload() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	var success bool
	if sf.downloadActive {
		sf.downloadActive = false
		success = true
	} else {
		success = false
	}
	return success
}
