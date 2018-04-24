package data

type ShareSubscriber interface {
	ReceivedDownloadRequest()
	ReceivedDownloadRequestResult(rr DownloadRequestResult)
	ReceivedShareList(sf SharedFile)
}
