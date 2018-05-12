package net

import (
	"github.com/google/uuid"
	"github.com/monz/fastSharerGo/net/data"
	"log"
)

type Sender interface {
	Send(cmdType data.Cmd, data []interface{}, remoteNodeId uuid.UUID, failMsg string)
	SendCallback(cmdType data.Cmd, data []interface{}, remoteNodeId uuid.UUID, callback data.Callback)
}

type ShareSender struct {
	sender chan data.ShareCommand
}

func NewShareSender(sender chan data.ShareCommand) *ShareSender {
	s := new(ShareSender)
	s.sender = sender

	return s
}

func (s *ShareSender) Send(cmdType data.Cmd, cmdData []interface{}, nodeId uuid.UUID, failMsg string) {
	cmd := data.NewShareCommand(cmdType, cmdData, nodeId, func() {
		// could not send data
		log.Println(failMsg)
		return
	})
	s.sender <- *cmd
}

func (s *ShareSender) SendCallback(cmdType data.Cmd, cmdData []interface{}, nodeId uuid.UUID, callback data.Callback) {
	cmd := data.NewShareCommand(cmdType, cmdData, nodeId, callback)
	s.sender <- *cmd
}
