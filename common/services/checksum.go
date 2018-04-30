package util

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"os"
)

type ChecksumService struct {
	workerToken chan int
}

func NewChecksumService(workerCount int) *ChecksumService {
	cs := new(ChecksumService)
	cs.workerToken = InitSema(workerCount)
	return cs
}

func NewHash() hash.Hash {
	return md5.New()
}

func CalculateChecksum(filePath string, offset int64, size int64) (checksum string, err error) {
	// take worker token
	//<-cs.workerToken
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
	// release worker token
	//workerToken <- 1
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
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
