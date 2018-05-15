package net

import (
	"fmt"
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/common/services"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createDownloader() (Downloader, chan data.ShareCommand) {
	downloadTokens := 1
	senderChan := make(chan data.ShareCommand)
	sender := NewShareSender(senderChan)
	return NewShareDownloader(uuid.New(), downloadTokens, sender), senderChan
}

func TestRequestAlreadyDownloadingIgnore(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	if ok := sf.ActivateDownload(); !ok {
		t.Error("Could not activate download for shared file")
	}
	downloader, senderChan := createDownloader()

	// request download of shared file
	filePath := filepath.Join(downloadDir, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)

	// expect no message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case <-senderChan:
			t.Error("Should not receive message")
			done <- true
		case <-time.After(250 * time.Millisecond):
			done <- true
		}
	}()

	// wait
	<-done
}

func TestRequestValidationActiveIgnore(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	if ok := sf.ActivateValidation(); !ok {
		t.Error("Could not activate validation for shared file")
	}
	downloader, senderChan := createDownloader()

	// request download of shared file
	filePath := filepath.Join(downloadDir, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)

	// expect no message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case <-senderChan:
			t.Error("Should not receive message")
			done <- true
		case <-time.After(250 * time.Millisecond):
			done <- true
		}
	}()

	// wait
	<-done
}

func TestRequestValidationActiveWaitingForChecksumIgnore(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	// file checksum not arrived yet
	checksum := sf.Checksum()
	sf.SetChecksum("")
	downloader, senderChan := createDownloader()

	// request download of shared file
	filePath := filepath.Join(base, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)
	// simulate receiving same shared file info
	time.Sleep(50 * time.Millisecond)
	go downloader.RequestDownload(sf, filePath)
	// simulate receiving same shared file info
	time.Sleep(50 * time.Millisecond)
	sf.SetChecksum(checksum)
	go downloader.RequestDownload(sf, filePath)

	// expect no message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case <-senderChan:
			t.Error("Should not receive message")
			done <- true
		case <-time.After(250 * time.Millisecond):
			done <- true
		}
	}()

	// wait
	<-done
}

func TestRequestFileExistsValidIgnore(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	downloader, senderChan := createDownloader()

	// request download of shared file
	filePath := filepath.Join(base, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)

	// expect no message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case <-senderChan:
			t.Error("Should not receive message")
			done <- true
		case <-time.After(250 * time.Millisecond):
			done <- true
		}
	}()

	// wait
	<-done
}

func TestRequestFileExistsNotValidDelteExisting(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	downloader, senderChan := createDownloader()

	// alter file checksum
	sf.SetChecksum(fmt.Sprintf("%x", util.NewHash().Sum(nil)))
	// no chunks downloaded yet
	sf.SetAllChunksLocal(false)
	// add replica node which shares the file
	replicaNode := commonData.NewReplicaNode(uuid.New(), sf.Chunks(), true)
	sf.UpdateReplicaNode(replicaNode)

	// request download of shared file
	filePath := filepath.Join(base, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)

	// wait for file deletion
	time.Sleep(100 * time.Millisecond)

	// expect file was deleted
	if _, err := os.Stat(sf.FilePath()); !os.IsNotExist(err) {
		t.Error("File still exists, should be deleted")
	}

	// expect download request message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case cmd := <-senderChan:
			// check cmd type
			if cmd.Type() != data.DownloadRequestCmd {
				t.Error("Expected download request message")
			}
			done <- true
		case <-time.After(500 * time.Millisecond):
			t.Error("Should not time out")
			done <- true
		}
	}()

	// wait
	<-done
}

func TestRequestFileLocalDownloadFinished(t *testing.T) {
	downloadDir, base := prepareDirs(t)
	defer os.RemoveAll(downloadDir)
	defer os.RemoveAll(base)
	sf := createSharedFile(t, base)
	defer os.Remove(sf.FilePath())
	downloader, senderChan := createDownloader()

	// all chunks local
	sf.SetAllChunksLocal(true)

	// request download of shared file
	filePath := filepath.Join(downloadDir, sf.FileRelativePath())
	go downloader.RequestDownload(sf, filePath)

	// expect no message on sender chan
	done := make(chan bool)
	go func() {
		select {
		case <-senderChan:
			t.Error("Should not receive message")
			done <- true
		case <-time.After(250 * time.Millisecond):
			done <- true
		}
	}()

	// wait
	<-done
}
