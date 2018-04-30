package data

import (
	"encoding/json"
	"github.com/google/uuid"
	"sync"
)

type ReplicaNode struct {
	id               uuid.UUID
	chunks           map[string]bool
	isComplete       bool
	isStopSharedInfo bool
	mu               sync.Mutex
}

// type is only for marshalling/unmarshalling
type replicaNode struct {
	Id         uuid.UUID `json:"id"`
	Chunks     []string  `json:"chunks"`
	IsComplete bool      `json:"isComplete"`
}

func NewReplicaNode(id uuid.UUID, chunks []string, isComplete bool) *ReplicaNode {
	node := new(ReplicaNode)
	node.id = id
	node.chunks = toMap(chunks)
	node.isComplete = isComplete

	return node
}

func (n ReplicaNode) MarshalJSON() ([]byte, error) {
	return json.Marshal(toReplicaNode(n))
}

func (n *ReplicaNode) UnmarshalJSON(b []byte) error {
	var node replicaNode
	err := json.Unmarshal(b, &node)
	if err == nil {
		fromReplicaNode(n, node.Id, node.Chunks, node.IsComplete)
	}
	return err
}

// helper func for marshalling/unmarshalling
func toReplicaNode(n ReplicaNode) replicaNode {
	var node replicaNode
	node.Id = n.id

	slice := make([]string, 0, len(n.chunks))
	for c, _ := range n.chunks {
		slice = append(slice, c)
	}
	node.Chunks = slice
	node.IsComplete = n.isComplete

	return node
}

// helper func for marshalling/unmarshalling
func fromReplicaNode(node *ReplicaNode, id uuid.UUID, chunks []string, isComplete bool) *ReplicaNode {
	node.id = id

	m := toMap(chunks)
	node.chunks = m
	node.isComplete = isComplete

	return node
}

func toMap(chunkChecksums []string) map[string]bool {
	m := make(map[string]bool)

	for _, c := range chunkChecksums {
		if len(c) > 0 {
			m[c] = true
		}
	}
	return m
}

func (n ReplicaNode) Id() uuid.UUID {
	return n.id
}

func (n *ReplicaNode) PutIfAbsent(chunkChecksum string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	// skip empty checksum
	if len(chunkChecksum) <= 0 {
		return false
	}

	absent := false
	_, ok := n.chunks[chunkChecksum]
	if !ok {
		n.chunks[chunkChecksum] = true
		absent = true
	}
	return absent
}

func (n *ReplicaNode) Chunks() map[string]bool {
	return n.chunks
}

func (n ReplicaNode) IsComplete() bool {
	return n.isComplete
}

func (n *ReplicaNode) SetIsComplete(isComplete bool) {
	n.isComplete = isComplete
}

func (n ReplicaNode) Contains(chunkChecksum string) bool {
	_, ok := n.chunks[chunkChecksum]
	return ok
}

func (n ReplicaNode) IsStopSharedInfo() bool {
	return n.isStopSharedInfo
}

func (n *ReplicaNode) SetStopSharedInfo(stopSharedInfo bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.isStopSharedInfo = stopSharedInfo
}