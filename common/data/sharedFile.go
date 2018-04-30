package data

import (
	"github.com/google/uuid"
	"log"
	"sync"
)

type SharedFile struct {
	FileMetadata     FileMetadata               `json:"metadata"`
	FileReplicaNodes map[uuid.UUID]*ReplicaNode `json:"replicaNodes"`
	downloadActive   bool
	mu               sync.Mutex
}

func NewSharedFile(metadata FileMetadata) *SharedFile {
	sf := new(SharedFile)
	sf.FileMetadata = metadata
	sf.FileReplicaNodes = make(map[uuid.UUID]*ReplicaNode)

	return sf
}

func (sf SharedFile) FileName() string {
	return sf.FileMetadata.Name()
}

func (sf SharedFile) FilePath() string {
	return sf.FileMetadata.Path()
}

func (sf SharedFile) FileRelativePath() string {
	return sf.FileMetadata.RelativePath()
}

func (sf *SharedFile) Checksum() string {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.FileMetadata.Checksum()
}

func (sf *SharedFile) SetChecksum(checksum string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.FileMetadata.SetChecksum(checksum)
}

func (sf SharedFile) IsLocal() bool {
	var isLocal bool
	expectedChunkCount := ChunkCount(sf.FileMetadata.Size())
	actualChunkCount := len(sf.FileMetadata.Chunks())
	log.Printf("In isLocal: expectedChunkCount = %d, actualChunkCount = %d\n", expectedChunkCount, actualChunkCount)
	if actualChunkCount > 0 && actualChunkCount == expectedChunkCount {
		isLocal = sf.FileMetadata.AllChunksLocal()
		log.Printf("All chunk local = %v\n", isLocal)
	} else {
		isLocal = false
	}
	return isLocal
}

func (sf SharedFile) FileId() string {
	return sf.FileMetadata.Id()
}

func (sf SharedFile) ReplicaNodes() map[uuid.UUID]*ReplicaNode {
	return sf.FileReplicaNodes
}

func (sf *SharedFile) ReplicaNodeById(id uuid.UUID) (ReplicaNode, bool) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	replicaNode, ok := sf.FileReplicaNodes[id]
	var nodeCopy ReplicaNode
	if ok {
		nodeCopy = *replicaNode
	}

	return nodeCopy, ok
}

func (sf *SharedFile) AddReplicaNode(newNode ReplicaNode) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	// put node if absent
	node, ok := sf.FileReplicaNodes[newNode.Id()]
	if !ok {
		sf.FileReplicaNodes[newNode.Id()] = &newNode
		return
	}
	// add new chunk checksums
	for chunkChecksum, _ := range newNode.Chunks() {
		// skip invalid checksums
		if len(chunkChecksum) <= 0 {
			continue
		}
		node.PutIfAbsent(chunkChecksum)
	}
}

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

func (sf *SharedFile) AddChunk(chunk *Chunk) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.FileMetadata.AddChunk(chunk)
}

func (sf *SharedFile) ClearReplicaNodes() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.FileReplicaNodes = make(map[uuid.UUID]*ReplicaNode)
}

func (sf *SharedFile) ClearChunksWithoutChecksum() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.FileMetadata.ClearChunksWithoutChecksum()
}

func (sf *SharedFile) Chunks() []*Chunk {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.FileMetadata.Chunks()
}

func (sf *SharedFile) ChunksToDownload() []*Chunk {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	var chunks []*Chunk
	if len(sf.FileMetadata.Chunks()) > 0 && !sf.IsLocal() {
		for _, c := range sf.FileMetadata.Chunks() {
			if !c.IsLocal() && !c.IsDownloadActive() && len(c.Checksum()) > 0 {
				chunks = append(chunks, c)
			}
		}
	}

	return chunks
}

func (sf *SharedFile) SetAllChunksLocal(isLocal bool) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	for _, c := range sf.FileMetadata.Chunks() {
		c.SetLocal(isLocal)
	}
}

func (sf SharedFile) ReplicaNodesByChunk(chunkChecksum string) []ReplicaNode {
	var replicaNodes []ReplicaNode
	for _, replicaNode := range sf.FileReplicaNodes {
		if replicaNode.Contains(chunkChecksum) {
			replicaNodes = append(replicaNodes, *replicaNode)
		}
	}

	return replicaNodes
}

func (sf SharedFile) ChunkById(chunkChecksum string) (*Chunk, bool) {
	return sf.FileMetadata.ChunkById(chunkChecksum)
}

func (sf SharedFile) LocalChunksChecksums() []string {
	return sf.FileMetadata.LocalChunksChecksums()
}
