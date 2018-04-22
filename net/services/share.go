package net

import (
	"log"
)

type ShareService struct {
}

func NewShareService() *ShareService {
	s := new(ShareService)

	return s
}

func (s ShareService) Start() {
}

func (s ShareService) Stop() {
}

func (s ShareService) DownloadRequest() {
	log.Println("Added download request")
}

func (s ShareService) DownloadRequestResult() {
	log.Println("Added upload")
}

func (s ShareService) PushShareList() {
	log.Println("Added shared file")
}
