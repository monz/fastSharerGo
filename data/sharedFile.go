package data

import (
        "sync"
        "github.com/monz/fastSharerGo/net/data"
        "github.com/google/uuid"
)

type SharedFile struct {
        metadata FileMetadata
        replicaNodes map[uuid.UUID]data.ReplicaNode // type to be done
        downloadActive bool
        mu sync.Mutex
}

func NewSharedFile(metadata FileMetadata) *SharedFile {
        sf := new(SharedFile)
        sf.metadata = metadata

        return sf
}

func (sf SharedFile) FilePath() string {
        return sf.metadata.FilePath()
}

func (sf SharedFile) SetFilePath(filePath string) {
        sf.metadata.SetFilePath(filePath)
}

func (sf SharedFile) Metadata() FileMetadata {
        return sf.metadata
}

func (sf SharedFile) Checksum() string {
        return sf.metadata.Checksum()
}

func (sf SharedFile) IsLocal() bool {
        var isLocal bool
        expectedChunkCount := GetChunkCount(sf.metadata.FileSize())
        actualChunkCount := sf.metadata.Chunks().Len()
        if actualChunkCount > 0 && actualChunkCount == expectedChunkCount {
                isLocal = sf.metadata.AllChunksLocal()
        } else {
                isLocal = false
        }
        return isLocal
}

func (sf SharedFile) FileId() string {
        return sf.metadata.FileId()
}

func (sf SharedFile) ReplicaNodes() map[uuid.UUID]data.ReplicaNode {
    return sf.replicaNodes
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
