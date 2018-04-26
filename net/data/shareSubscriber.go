package data

type ShareSubscriber interface {
	ReceivedDownloadRequest(r DownloadRequest)
	ReceivedDownloadRequestResult(rr DownloadRequestResult)
	ReceivedShareList(sf SharedFile)
}
