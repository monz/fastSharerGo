package data

type ShareSubscriber interface {
	DownloadRequest()
	DownloadRequestResult()
	PushShareList()
}
