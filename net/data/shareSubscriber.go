package data

import (
	"github.com/monz/fastSharerGo/common/data"
)

type ShareSubscriber interface {
	ReceivedDownloadRequest(r DownloadRequest)
	ReceivedDownloadRequestResult(rr DownloadRequestResult)
	ReceivedShareList(sf data.SharedFile)
}
