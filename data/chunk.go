package data

import (
    "math"
    "container/list"
    "log"
)

const CHUNK_SIZE = 1024*1024*64 // 64 MByte

type Chunk struct {
    checksum string
    offset int64
    size int64
    fileId string
    isLocal bool
    downloadActive bool
}

func NewChunk(fileId string, offset int64, size int64) *Chunk {
    c := new(Chunk)
    c.fileId = fileId
    c.offset = offset
    c.size = size
    c.isLocal = true;
    c.downloadActive = false;

    return c
}

func (c Chunk) FileId() string {
        return c.fileId
}

func (c Chunk) SetFileId(fileId string) {
        c.fileId = fileId
}

func (c Chunk) SetLocal(isLocal bool) {
        c.isLocal = isLocal
}

func (c Chunk) ActivateDownload() bool {
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

func (c Chunk) DeactivateDownload() bool {
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

func (c Chunk) IsDownloadActive() bool {
        return c.downloadActive
}

func (c Chunk) IsLocal() bool {
        return c.isLocal
}

func (c Chunk) Checksum() string {
        return c.checksum
}

func (c Chunk) SetChecksum(checksum string) {
        c.checksum = checksum
}

func (c Chunk) Offset() int64 {
        return c.offset
}

func (c Chunk) Size() int64 {
        return c.size
}

func (c Chunk) Equals(c2 Chunk) bool {
        return c.checksum == c2.Checksum()
}

func (c Chunk) HasChecksum() bool {
        return len(c.checksum) > 0
}

func (c Chunk) RequestAnswered() {
        // todo: implement
        log.Println("requestAnswered is not implemented!")
        // waitSince = -1
}

func GetChunkCount(fileSize int64) int {
        return int(math.Ceil(float64(fileSize) / CHUNK_SIZE))
}

func GetChunks(fileId string, fileSize int64) *list.List {
        chunks := list.New()

        remainingSize := fileSize
        size := int64(math.Min(CHUNK_SIZE, float64(remainingSize)))
        for size > 0 {
                offset := fileSize - remainingSize
                chunks.PushBack(NewChunk(fileId, offset, size))
                remainingSize -= int64(size)
                size = CHUNK_SIZE
                if (remainingSize - CHUNK_SIZE) < 0 {
                        size = int64(remainingSize)
                }
        }
        return chunks
}
