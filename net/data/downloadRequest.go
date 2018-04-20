package data

type DownloadRequest struct {
        fileId string
        nodeId string
        chunkChecksum string
}

func NewDownloadRequest(fileId string, nodeId string, chunkChecksum string) *DownloadRequest {
        request := new(DownloadRequest)
        request.fileId = fileId
        request.nodeId = nodeId
        request.chunkChecksum = chunkChecksum

        return request
}

func (r DownloadRequest) FileId() string {
        return r.fileId
}

func (r DownloadRequest) NodeId() string {
        return r.nodeId
}

func (r DownloadRequest) ChunkChecksum() string {
        return r.chunkChecksum
}
