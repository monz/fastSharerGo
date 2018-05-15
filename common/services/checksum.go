package util

import (
	"crypto/md5"
	"fmt"
	commonData "github.com/monz/fastSharerGo/common/data"
	"hash"
	"io"
	"log"
	"os"
)

type ChecksumService interface {
	SetFileChecksums(*commonData.FileMetadata)
}

type ShareChecksumService struct {
	workerToken chan int
}

func NewShareChecksumService(workerCount int) ChecksumService {
	cs := new(ShareChecksumService)
	cs.workerToken = InitSema(workerCount)
	return cs
}

func NewHash() hash.Hash {
	return md5.New()
}

func (cs *ShareChecksumService) SetFileChecksums(f *commonData.FileMetadata) {
	go func() {
		<-cs.workerToken
		fileChecksum, chunks := FileChecksums(f.Id(), f.Path(), f.Size())
		f.SetChecksum(fileChecksum)
		for _, c := range chunks {
			f.AddChunk(c)
		}
		cs.workerToken <- 1
	}()
}

func FileChecksums(fileId string, filePath string, fileSize int64) (string, []*commonData.Chunk) {
	chunks := commonData.GetChunks(fileId, fileSize)
	checksums := make([]string, 0, len(chunks))
	for _, c := range chunks {
		checksum, err := chunkChecksum(filePath, c.Offset(), c.Size())
		if err != nil {
			log.Fatal(err)
		}
		c.SetChecksum(checksum)
		c.SetLocal(true)
		checksums = append(checksums, checksum)
	}
	return SumOfChecksums(checksums), chunks
}

func chunkChecksum(filePath string, offset int64, size int64) (checksum string, err error) {
	// open file
	file, err := os.Open(filePath)
	if err != nil {
		return checksum, err
	}
	defer file.Close()
	hash := NewHash()
	// jump to offset
	sectionReader := io.NewSectionReader(file, offset, size)
	// calculate checksum
	_, err = io.Copy(hash, sectionReader)
	if err != nil {
		return checksum, err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func CompareFileChecksums(fileId string, filePath string, fileSize int64, expectedChecksum string) bool {
	calculatedChecksum, _ := FileChecksums(fileId, filePath, fileSize)

	log.Printf("FileComplete: expectedSum = %s, actualSum = %s\n", expectedChecksum, calculatedChecksum)
	return expectedChecksum == calculatedChecksum
}

func SumOfChecksums(checksums []string) string {
	completeHash := NewHash()
	for _, sum := range checksums {
		io.WriteString(completeHash, sum)
	}
	return fmt.Sprintf("%x", completeHash.Sum(nil))
}

func InitSema(n int) chan int {
	channel := make(chan int, n)
	for i := 0; i < n; i++ {
		channel <- 1
	}
	return channel
}
