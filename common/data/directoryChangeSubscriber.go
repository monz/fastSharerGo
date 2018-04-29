package data

type DirectoryChangeSubscriber interface {
	AddLocalSharedFile(sf SharedFile)
}
