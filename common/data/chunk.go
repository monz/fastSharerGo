package data

import (
	"math"
	"sync"
)

const CHUNK_SIZE = 1024 * 1024 * 64 // 64 MByte

type Chunk struct {
	ChunkChecksum  string `json:"checksum"`
	ChunkOffset    int64  `json:"offset"`
	ChunkSize      int64  `json:"size"`
	fileId         string
	isLocal        bool
	downloadActive bool
	mu             sync.Mutex
}

func NewChunk(fileId string, offset int64, size int64) *Chunk {
	c := new(Chunk)
	c.fileId = fileId
	c.ChunkOffset = offset
	c.ChunkSize = size
	c.isLocal = true
	c.downloadActive = false

	return c
}

func (c Chunk) FileId() string {
	return c.fileId
}

func (c Chunk) SetFileId(fileId string) {
	c.fileId = fileId
}

func (c *Chunk) SetLocal(isLocal bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isLocal = isLocal
}

// use semaphores to make method atomic!!!
func (c *Chunk) ActivateDownload() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	var success bool
	if c.downloadActive {
		success = false
	} else {
		c.downloadActive = true
		success = true
		// waitSince = System.currentTimeMillis();
	}
	return success
}

// use semaphores to make method atomic!!!
func (c *Chunk) DeactivateDownload() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	var success bool
	if c.downloadActive {
		c.downloadActive = false
		success = true
		// waitSince = -1
	} else {
		success = false
	}
	return success
}

func (c *Chunk) IsDownloadActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.downloadActive
}

func (c Chunk) IsLocal() bool {
	return c.isLocal
}

func (c Chunk) Checksum() string {
	return c.ChunkChecksum
}

func (c Chunk) Offset() int64 {
	return c.ChunkOffset
}

func (c Chunk) Size() int64 {
	return c.ChunkSize
}

func (c Chunk) HasChecksum() bool {
	return len(c.ChunkChecksum) > 0
}

func ChunkCount(fileSize int64) int {
	return int(math.Ceil(float64(fileSize) / CHUNK_SIZE))
}

func GetChunks(fileId string, fileSize int64) []*Chunk {
	var chunks []*Chunk

	remainingSize := fileSize
	size := int64(math.Min(CHUNK_SIZE, float64(remainingSize)))
	for size > 0 {
		offset := fileSize - remainingSize
		chunk := NewChunk(fileId, offset, size)
		chunks = append(chunks, chunk)
		remainingSize -= int64(size)
		size = CHUNK_SIZE
		if (remainingSize - CHUNK_SIZE) < 0 {
			size = int64(remainingSize)
		}
	}
	return chunks
}
