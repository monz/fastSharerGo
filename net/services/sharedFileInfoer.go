package net

import (
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/net/data"
	"log"
)

type FileInfoer interface {
	SendFileInfo(commonData.SharedFile, uuid.UUID)
	SendCompleteMsg(commonData.SharedFile, uuid.UUID)
}

type SharedFileInfoer struct {
	localNodeId uuid.UUID
	sender      chan data.ShareCommand
}

func NewSharedFileInfoer(localNodeId uuid.UUID, sender chan data.ShareCommand) *SharedFileInfoer {
	s := new(SharedFileInfoer)
	s.localNodeId = localNodeId
	s.sender = sender

	return s
}

func (s *SharedFileInfoer) SendFileInfo(sf commonData.SharedFile, nodeId uuid.UUID) {
	// add local node as replica node for all local chunks
	chunkSums := sf.LocalChunksChecksums()
	localNode := commonData.NewReplicaNode(s.localNodeId, chunkSums, sf.IsLocal())
	sf.AddReplicaNode(localNode)

	// send data
	shareList := []interface{}{sf}
	cmd := data.NewShareCommand(data.PushShareListCmd, shareList, nodeId, func() {
		// could not send data
		log.Println("Could not send share list data")
		return
	})
	s.sender <- *cmd
}

func (s *SharedFileInfoer) SendCompleteMsg(sf commonData.SharedFile, nodeId uuid.UUID) {
	// send information that node is complete
	localNode := commonData.NewReplicaNode(s.localNodeId, []string{}, len(sf.Checksum()) > 0)
	sf.AddReplicaNode(localNode)

	// send data
	shareList := []interface{}{sf}
	cmd := data.NewShareCommand(data.PushShareListCmd, shareList, nodeId, func() {
		// could not send data
		log.Println("Could not send complete message")
		return
	})
	s.sender <- *cmd
}
