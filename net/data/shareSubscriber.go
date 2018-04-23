package data

type ShareSubscriber interface {
	ReceivedDownloadRequest()
	ReceivedDownloadRequestResult()
	ReceivedShareList(sf SharedFile)
}
