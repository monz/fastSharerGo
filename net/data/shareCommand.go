package data

import (
        "container/list"
)

// https://stackoverflow.com/questions/14426366/what-is-an-idiomatic-way-of-representing-enums-in-go?utm_medium=organic&utm_source=google_rich_qa&utm_campaign=google_rich_qa
type cmd int

const (
        PushShareListCmd cmd = iota
        DownloadRequestCmd
        DownloadRequestResultCmd
)

type ShareCommand interface {
    Type() cmd
    Data() *list.List
    Serialize()
    Deserialize()
}
