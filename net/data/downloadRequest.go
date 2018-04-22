package data

type DownloadRequest struct {
	RequestFileId        string `json:"fileId"`
	RequestNodeId        string `json:"nodeId"`
	RequestChunkChecksum string `json:"chunkChecksum"`
}

func NewDownloadRequest(fileId string, nodeId string, chunkChecksum string) *DownloadRequest {
	request := new(DownloadRequest)
	request.RequestFileId = fileId
	request.RequestNodeId = nodeId
	request.RequestChunkChecksum = chunkChecksum

	return request
}

func (r DownloadRequest) FileId() string {
	return r.RequestFileId
}

func (r DownloadRequest) NodeId() string {
	return r.RequestNodeId
}

func (r DownloadRequest) ChunkChecksum() string {
	return r.RequestChunkChecksum
}
