package data

type DownloadRequestResult struct {
	RequestFileId        string `json:"fileId"`
	RequestNodeId        string `json:"nodeId"`
	RequestChunkChecksum string `json:"chunkChecksum"`
	RequestDownloadPort  int    `json:"downloadPort"`
}

func NewDownloadRequestResult(fileId string, nodeId string, chunkChecksum string, downloadPort int) *DownloadRequestResult {
	request := new(DownloadRequestResult)
	request.RequestFileId = fileId
	request.RequestNodeId = nodeId
	request.RequestChunkChecksum = chunkChecksum
	request.RequestDownloadPort = downloadPort

	return request
}

func (r DownloadRequestResult) FileId() string {
	return r.RequestFileId
}

func (r DownloadRequestResult) NodeId() string {
	return r.RequestNodeId
}

func (r DownloadRequestResult) ChunkChecksum() string {
	return r.RequestChunkChecksum
}

func (r DownloadRequestResult) DownloadPort() int {
	return r.RequestDownloadPort
}
