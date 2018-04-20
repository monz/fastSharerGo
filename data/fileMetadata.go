package data

import (
        "container/list"
        "github.com/google/uuid"
        "os"
        "log"
)

type FileMetadata struct {
        fileId string
        fileSize int64
        checksum string
        fileName string
        chunks *list.List
        relativePath string
        filePath string
}

//func New(filePath string) *FileMetadata {
//        return newFileMetadata(uuid.New(), filePath, "")
//}

func NewFileMetadata(filePath string, relativePath string) *FileMetadata {
        return newFileMetadata(uuid.New().String(), filePath, relativePath)
}

func newFileMetadata(fileId string, filePath string, relativePath string) *FileMetadata {
        f := new(FileMetadata)

        f.fileId = fileId
        f.filePath = filePath
        f.relativePath = relativePath

        file, err := os.Open(filePath)
        if err != nil {
                log.Fatal(err)
        }
        fileInfo, err := file.Stat()
        if err != nil {
                log.Fatal(err)
        }

        f.fileName = fileInfo.Name()
        f.fileSize = fileInfo.Size()
        f.chunks = GetChunks(f.fileId, f.fileSize)

        return f
}

func (f FileMetadata) Chunks() *list.List {
        return f.chunks
}

func (f FileMetadata) IsChunkLocal(checksum string) bool {
        // todo: implement
        return false
}

func (f FileMetadata) Checksum() string {
        return f.checksum
}

func (f FileMetadata) SetChecksum(checksum string) {
        f.checksum = checksum
}

func (f FileMetadata) FilePath() string {
        return f.filePath
}

func (f FileMetadata) SetFilePath(filePath string) {
        f.filePath = filePath
}

func (f FileMetadata) FileSize() int64 {
        return f.fileSize
}

func (f FileMetadata) FileId() string {
        return f.fileId
}

func (f FileMetadata) HasChecksum() bool {
        return len(f.checksum) > 0
}

func (f FileMetadata) RelativePath() string {
        return f.relativePath
}

func (f FileMetadata) AllChunksLocal() bool {
        allChunksLocal := true
        chunks := f.Chunks()
        for e := chunks.Front(); e != nil; e = e.Next() {
                value, ok := e.Value.(Chunk)
                if !ok {
                        log.Println("No chunk data!")
                        continue
                }
                if !value.IsLocal() {
                        allChunksLocal = false
                        break
                }
        }
        return allChunksLocal
}
