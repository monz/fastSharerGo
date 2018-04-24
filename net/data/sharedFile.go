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
	expectedChunkCount := data.ChunkCount(sf.FileMetadata.Size())
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
func (sf *SharedFile) ActivateDownload() bool {
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
func (sf *SharedFile) DeactivateDownload() bool {
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

func (sf *SharedFile) IsDownloadActive() bool {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.downloadActive
}

func (sf *SharedFile) ChunksToDownload() []*data.Chunk {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	var chunks []*data.Chunk
	if len(sf.FileMetadata.Chunks()) > 0 && !sf.IsLocal() {
		for _, c := range sf.FileMetadata.Chunks() {
			if !c.IsLocal() && !c.IsDownloadActive() && len(c.Checksum()) > 0 {
				chunks = append(chunks, c)
			}
		}
	}

	return chunks
}

func (sf SharedFile) ReplicaNodesByChunk(chunkChecksum string) []ReplicaNode {
	var replicaNodes []ReplicaNode
	for _, replicaNode := range sf.FileReplicaNodes {
		if replicaNode.Contains(chunkChecksum) {
			replicaNodes = append(replicaNodes, replicaNode)
		}
	}

	return replicaNodes
}

func (sf SharedFile) ChunkById(chunkChecksum string) (*data.Chunk, error) {
	return sf.FileMetadata.ChunkById(chunkChecksum)
}
