package session

import (
	"fmt"
	"iter"
)

type LLMMessageTreeNode struct {
	Message       *LLMMessage `json:"message"`
	ChildrenIndex []int       `json:"children_index"`
	ParentIndex   int         `json:"parent_index"`
}

type LLMMessageTree struct {
	Nodes []*LLMMessageTreeNode `json:"nodes"`
}

const RootParentIndex = -1

/*
新增节点，返回节点索引。如果parentIndex小于0或超出范围时，则作为根节点。
*/
func (t *LLMMessageTree) AddNode(message *LLMMessage, parentIndex int) int {
	node := &LLMMessageTreeNode{
		Message:       message,
		ChildrenIndex: []int{},
	}
	t.Nodes = append(t.Nodes, node)
	if parentIndex != RootParentIndex && parentIndex < len(t.Nodes) && parentIndex >= 0 {
		t.Nodes[parentIndex].ChildrenIndex = append(t.Nodes[parentIndex].ChildrenIndex, len(t.Nodes)-1)
		node.ParentIndex = parentIndex
		return len(t.Nodes) - 1
	}
	node.ParentIndex = RootParentIndex
	return len(t.Nodes) - 1
}

/*
从某个节点向上遍历
*/
func (t *LLMMessageTree) IterFromChild(index int) iter.Seq2[int, *LLMMessage] {
	if index < 0 || index >= len(t.Nodes) {
		return func(yield func(int, *LLMMessage) bool) {}
	}
	return func(yield func(int, *LLMMessage) bool) {
		for i := index; i < len(t.Nodes) && i >= 0; {
			if !yield(i, t.Nodes[i].Message) {
				return
			}
			i = t.Nodes[i].ParentIndex
			if i == RootParentIndex {
				return
			}
		}
	}
}

func (t *LLMMessageTree) GetParentIndex(index int) (int, error) {
	if index < 0 || index >= len(t.Nodes) {
		return 0, fmt.Errorf("index out of range")
	}
	return t.Nodes[index].ParentIndex, nil
}

func (t *LLMMessageTree) GetNode(index int) (*LLMMessage, error) {
	if index < 0 || index >= len(t.Nodes) {
		return nil, fmt.Errorf("index out of range")
	}
	return t.Nodes[index].Message, nil
}
