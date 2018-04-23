package data

type NodeSubscriber interface {
	AddNode(n Node)
	RemoveNode(n Node)
}
