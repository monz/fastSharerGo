package net

import (
	"github.com/google/uuid"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/net/data"
)

type FileInfoer interface {
	SendFileInfo(commonData.SharedFile, uuid.UUID)
	SendCompleteMsg(commonData.SharedFile, uuid.UUID)
}

type SharedFileInfoer struct {
	localNodeId uuid.UUID
	sender      Sender
}

func NewSharedFileInfoer(localNodeId uuid.UUID, sender Sender) *SharedFileInfoer {
	s := new(SharedFileInfoer)
	s.localNodeId = localNodeId
	s.sender = sender

	return s
}

func (s *SharedFileInfoer) SendFileInfo(sf commonData.SharedFile, nodeId uuid.UUID) {
	// add local node as replica node for all local chunks
	chunkSums := sf.LocalChunksChecksums()
	// only send shared file info containing chunk information
	if len(chunkSums) <= 0 {
		return
	}
	localNode := commonData.NewReplicaNode(s.localNodeId, chunkSums, sf.IsLocal())
	sf.UpdateReplicaNode(localNode)

	// send data
	s.sender.Send(data.PushShareListCmd, []interface{}{sf}, nodeId, "Could not send shared file info")
}

func (s *SharedFileInfoer) SendCompleteMsg(sf commonData.SharedFile, nodeId uuid.UUID) {
	// send information that node is complete
	localNode := commonData.NewReplicaNode(s.localNodeId, []string{}, len(sf.Checksum()) > 0)
	sf.UpdateReplicaNode(localNode)

	// send data
	s.sender.Send(data.PushShareListCmd, []interface{}{sf}, nodeId, "Could not send complete message")
}
