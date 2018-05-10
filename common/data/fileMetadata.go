package data

import (
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/common/services"
	"log"
	"os"
	"sync"
)

// have to use exported members for JSON unmarshalling
type FileMetadata struct {
	FileId           string   `json:"fileId"`
	FileSize         int64    `json:"fileSize"`
	FileChecksum     string   `json:"checksum"`
	FileName         string   `json:"fileName"`
	FileChunks       []*Chunk `json:"chunks"`
	FileRelativePath string   `json:"relativePath"`
	FilePath         string   `json:"filePath"`
	mu               sync.Mutex
}

func NewFileMetadata(filePath string, relativePath string) *FileMetadata {
	return newFileMetadata(uuid.New().String(), filePath, relativePath)
}

func newFileMetadata(fileId string, filePath string, relativePath string) *FileMetadata {
	f := new(FileMetadata)

	f.FileId = fileId
	f.FilePath = filePath
	f.FileRelativePath = relativePath

	fileInfo, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err) // todo: handle error properly, don't stop programm
	}

	f.FileName = fileInfo.Name()
	f.FileSize = fileInfo.Size()
	f.FileChunks = GetChunks(f.FileId, f.FileSize)
	go f.calculateChecksums()

	return f
}

func (f *FileMetadata) calculateChecksums() {
	f.mu.Lock()
	defer f.mu.Unlock()

	checksums := make([]string, 0, len(f.FileChunks))
	for _, c := range f.FileChunks {
		checksum, err := util.CalculateChecksum(f.FilePath, c.Offset(), c.Size())
		if err != nil {
			log.Fatal(err)
		}
		c.SetChecksum(checksum)
		c.SetLocal(true)
		checksums = append(checksums, checksum)
	}
	f.FileChecksum = util.SumOfChecksums(checksums)
}

func (f FileMetadata) Name() string {
	return f.FileName
}

func (f *FileMetadata) AddChunk(chunk *Chunk) {
	f.mu.Lock()
	defer f.mu.Unlock()
	isAbsent := true
	for _, c := range f.FileChunks {
		if c.Checksum() == chunk.Checksum() {
			isAbsent = false
			break
		}
	}
	if isAbsent {
		f.FileChunks = append(f.FileChunks, chunk)
	}
}

func (f *FileMetadata) ClearChunksWithoutChecksum() {
	f.mu.Lock()
	defer f.mu.Unlock()

	var cleaned []*Chunk
	for _, c := range f.FileChunks {
		if len(c.Checksum()) > 0 {
			cleaned = append(cleaned, c)
		}
	}

	f.FileChunks = cleaned
}

func (f FileMetadata) Chunks() []*Chunk {
	return f.FileChunks
}

func (f FileMetadata) Checksum() string {
	return f.FileChecksum
}

func (f *FileMetadata) SetChecksum(checksum string) {
	f.FileChecksum = checksum
}

func (f FileMetadata) Path() string {
	return f.FilePath
}

func (f *FileMetadata) SetFilePath(filePath string) {
	f.FilePath = filePath
}

func (f FileMetadata) Size() int64 {
	return f.FileSize
}

func (f FileMetadata) Id() string {
	return f.FileId
}

func (f FileMetadata) HasChecksum() bool {
	return len(f.FileChecksum) > 0
}

func (f FileMetadata) RelativePath() string {
	return f.FileRelativePath
}

func (f *FileMetadata) SetFileRelativePath(relativePath string) {
	f.FileRelativePath = relativePath
}

func (f FileMetadata) AllChunksLocal() bool {
	allChunksLocal := true
	for _, c := range f.FileChunks {
		if !c.IsLocal() {
			allChunksLocal = false
			break
		}
	}
	return allChunksLocal
}

func (f FileMetadata) ChunkById(chunkChecksum string) (chunk *Chunk, ok bool) {
	for _, c := range f.FileChunks {
		if c.Checksum() == chunkChecksum {
			chunk = c
			ok = true
			break
		}
	}
	return chunk, ok
}

func (f FileMetadata) LocalChunksChecksums() []string {
	sums := make([]string, 0, len(f.FileChunks))
	for _, c := range f.FileChunks {
		if c.IsLocal() {
			sums = append(sums, c.Checksum())
		}
	}
	return sums
}
