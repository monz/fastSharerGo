package net

import (
	commonData "github.com/monz/fastSharerGo/common/data"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createSharedFile(t *testing.T, parentDir string) *commonData.SharedFile {
	tmpFile := createFile(t, parentDir)
	relativePath, err := filepath.Rel(parentDir, tmpFile.Name())
	if err != nil {
		t.Error(err)
	}
	meta := commonData.NewFileMetadata(tmpFile.Name(), relativePath)
	sf := commonData.NewSharedFile(meta)

	// wait for chunk calculation
	for len(sf.Checksum()) <= 0 {
		time.Sleep(10 * time.Millisecond)
	}

	return sf
}

func createFile(t *testing.T, parentDir string) *os.File {
	file, err := ioutil.TempFile(parentDir, "sharer")
	if err != nil {
		t.Error(err)
	}
	data := []byte("test file content")
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < rand.Intn(100)+1; i++ {
		if _, err := file.Write(data); err != nil {
			t.Error(err)
		}
	}
	if err = file.Close(); err != nil {
		t.Error(err)
	}
	return file
}

func prepareDirs(t *testing.T) (string, string) {
	downloadDir, err := ioutil.TempDir("", "sharer")
	if err != nil {
		t.Error(err)
	}
	base, err := ioutil.TempDir("", "sharer")
	if err != nil {
		t.Error(err)
	}

	return downloadDir, base
}
