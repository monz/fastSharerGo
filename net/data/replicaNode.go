package data

import (
        "container/list"
        "github.com/google/uuid"
)

type ReplicaNode struct {
        id uuid.UUID
        chunks map[string]bool
        isComplete bool
        isStopSharedInfo bool
}

func NewReplicaNode(id uuid.UUID, chunks *list.List, isComplete bool) *ReplicaNode {
        node := new(ReplicaNode)
        node.id = id
        node.chunks = toMap(chunks)
        node.isComplete = isComplete

        return node
}

func toMap(l *list.List) map[string]bool {
        m := make(map[string]bool)

        for i := l.Front(); i != nil; i = i.Next() {
                value, ok := i.Value.(string)
                if ok {
                    m[value] = true
                }
        }
        return m
}

func (n ReplicaNode) Chunks() map[string]bool {
        return n.chunks
}

func (n ReplicaNode) IsComplete() bool {
        return n.isComplete
}

func (n ReplicaNode) Contains(chunkChecksum string) bool {
        _, ok := n.chunks[chunkChecksum]
        return ok
}

func (n ReplicaNode) IsStopSharedInfo() bool {
        return n.isStopSharedInfo
}

func (n ReplicaNode) StopSharedInfo(stopSharedInfo bool) {
        n.isStopSharedInfo = stopSharedInfo
}

