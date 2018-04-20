package data

type DownloadRequestResult struct {
        fileId string
        nodeId string
        chunkChecksum string
        downloadPort int
}

func NewDownloadRequestResult(fileId string, nodeId string, chunkChecksum string, downloadPort int) *DownloadRequestResult {
        request := new(DownloadRequestResult)
        request.fileId = fileId
        request.nodeId = nodeId
        request.chunkChecksum = chunkChecksum
        request.downloadPort = downloadPort

        return request
}

func (r DownloadRequestResult) FileId() string {
        return r.fileId
}

func (r DownloadRequestResult) NodeId() string {
        return r.nodeId
}

func (r DownloadRequestResult) ChunkChecksum() string {
        return r.chunkChecksum
}

func (r DownloadRequestResult) DownloadPort() int {
        return r.downloadPort
}
