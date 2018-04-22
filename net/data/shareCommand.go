package data

import (
	"container/list"
	"encoding/json"
	"errors"
	"log"
)

// https://stackoverflow.com/questions/14426366/what-is-an-idiomatic-way-of-representing-enums-in-go?utm_medium=organic&utm_source=google_rich_qa&utm_campaign=google_rich_qa
type Cmd int

const (
	PushShareListCmd Cmd = iota
	DownloadRequestCmd
	DownloadRequestResultCmd
)

func (c Cmd) MarshalJSON() ([]byte, error) {
	cmd, err := c.ToString()
	if err != nil {
		return nil, err
	}
	return json.Marshal(cmd)
}

func (c Cmd) ToString() (string, error) {
	switch c {
	case PushShareListCmd:
		return "PUSH_SHARE_LIST", nil
	case DownloadRequestCmd:
		return "DOWNLOAD_REQUEST", nil
	case DownloadRequestResultCmd:
		return "DOWNLOAD_REQUEST_RESULT", nil
	default:
		return "", errors.New("Unknown command")
	}
}

func (c *Cmd) ParseString(cmd string) error {
	var err error
	switch cmd {
	case "PUSH_SHARE_LIST":
		*c = PushShareListCmd
	case "DOWNLOAD_REQUEST":
		*c = DownloadRequestCmd
	case "DOWNLOAD_REQUEST_RESULT":
		*c = DownloadRequestResultCmd
	default:
		err = errors.New("Could not parse command")
	}
	return err

}

func (c *Cmd) UnmarshalJSON(b []byte) error {
	var cmdString string
	err := json.Unmarshal(b, cmdString)
	if err != nil {
		return err
	}
	return c.ParseString(cmdString)
}

type ShareCommand struct {
	cmdType Cmd
	data    *list.List
}

func NewShareCommand() *ShareCommand {
	cmd := new(ShareCommand)
	cmd.data = list.New()

	return cmd
}

func (c ShareCommand) Type() Cmd {
	return c.cmdType
}

func (c ShareCommand) Data() *list.List {
	return c.data
}

func (c ShareCommand) Serialize() {

}

func (c *ShareCommand) Deserialize(b []byte) error {
	// get first json layer
	var objMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &objMap)
	if err != nil {
		return err
	}

	// get cmd type
	var cmdType string
	value, ok := objMap["cmd"]
	if !ok {
		return errors.New("Unrecognized message type")
	}
	err = json.Unmarshal(*value, &cmdType)
	if err != nil {
		return err
	}
	if err := c.cmdType.ParseString(cmdType); err != nil {
		return err
	}
	log.Println("Could read command type", cmdType)

	// get elements of 'data' list
	var dataElements []*json.RawMessage
	// check whether data element exists
	value, ok = objMap["data"]
	if !ok {
		return errors.New("Unrecognized message type")
	}
	err = json.Unmarshal(*value, &dataElements)
	if err != nil {
		return err
	}

	// store data in list
	for _, rawMessage := range dataElements {
		switch c.Type() {
		case PushShareListCmd:
			var v SharedFile
			err := json.Unmarshal(*rawMessage, &v)
			if err != nil {
				return err
			}
			c.data.PushBack(v)
		case DownloadRequestCmd:
			var v DownloadRequest
			err := json.Unmarshal(*rawMessage, &v)
			if err != nil {
				return err
			}
			c.data.PushBack(v)
		case DownloadRequestResultCmd:
			var v DownloadRequestResult
			err := json.Unmarshal(*rawMessage, &v)
			if err != nil {
				return err
			}
			c.data.PushBack(v)
		default:
			err := errors.New("Should never happen")
			if err != nil {
				return err
			}
		}
	}
	return nil
}
