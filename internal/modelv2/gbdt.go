package modelv2

import (
	"errors"
	"fmt"
	"math"
)

const leafFeatureIndex = -1

// TreeNode represents a single node inside a flattened GBDT decision tree.
// Leaf nodes have Feature == -1 and return Leaf. Internal nodes branch on
// val < Threshold -> Left, val >= Threshold -> Right, and NaN/missing -> Missing.
type TreeNode struct {
	Feature   int     `json:"feature"`
	Threshold float64 `json:"threshold"`
	Left      int     `json:"left"`
	Right     int     `json:"right"`
	Missing   int     `json:"missing"`
	Leaf      float64 `json:"leaf"`
}

// FlatTree is a contiguous array of TreeNode values rooted at index 0.
type FlatTree struct {
	Nodes []TreeNode `json:"nodes"`
}

// GBDTModel is an additive ensemble of flattened regression trees rooted at BaseScore.
type GBDTModel struct {
	BaseScore float64    `json:"base_score"`
	Trees     []FlatTree `json:"trees"`
}

// Eval evaluates the GBDT ensemble on an aligned feature vector with zero heap
// allocations. Missing features should be encoded as math.NaN() (or omitted
// past len(features)), which routes through each split node's Missing branch.
func (m GBDTModel) Eval(features []float64) float64 {
	sum := m.BaseScore
	for i := range m.Trees {
		sum += m.Trees[i].Eval(features)
	}
	return sum
}

// Eval evaluates a single flattened tree on an aligned feature vector with zero
// heap allocations.
func (t FlatTree) Eval(features []float64) float64 {
	n := len(t.Nodes)
	if n == 0 {
		return 0
	}
	idx := 0
	for range n {
		node := &t.Nodes[idx]
		if node.Feature < 0 {
			return node.Leaf
		}
		switch {
		case node.Feature >= len(features) || math.IsNaN(features[node.Feature]):
			idx = node.Missing
		case features[node.Feature] < node.Threshold:
			idx = node.Left
		default:
			idx = node.Right
		}
		if idx < 0 || idx >= n {
			return 0
		}
	}
	return 0
}

// Validate checks that BaseScore is finite and every tree in the ensemble is
// non-empty, acyclic, and references valid feature and child indices.
func (m GBDTModel) Validate(numFeatures int) error {
	if math.IsNaN(m.BaseScore) || math.IsInf(m.BaseScore, 0) {
		return fmt.Errorf("base_score %v must be finite", m.BaseScore)
	}
	if len(m.Trees) == 0 {
		return errors.New("trees must not be empty")
	}
	for i := range m.Trees {
		if err := m.Trees[i].Validate(numFeatures); err != nil {
			return fmt.Errorf("trees[%d]: %w", i, err)
		}
	}
	return nil
}

// Validate checks that the flattened tree has at least one node, all feature
// indices and child pointers are in bounds, thresholds/leaves are finite, and
// no cycles exist reachable from root 0.
func (t FlatTree) Validate(numFeatures int) error {
	n := len(t.Nodes)
	if n == 0 {
		return errors.New("tree has no nodes")
	}
	for i, node := range t.Nodes {
		if node.Feature == leafFeatureIndex {
			if math.IsNaN(node.Leaf) || math.IsInf(node.Leaf, 0) {
				return fmt.Errorf("node %d leaf %v must be finite", i, node.Leaf)
			}
			continue
		}
		if node.Feature < 0 || node.Feature >= numFeatures {
			return fmt.Errorf("node %d feature %d out of [0, %d)", i, node.Feature, numFeatures)
		}
		if math.IsNaN(node.Threshold) || math.IsInf(node.Threshold, 0) {
			return fmt.Errorf("node %d threshold %v must be finite", i, node.Threshold)
		}
		if node.Left < 0 || node.Left >= n {
			return fmt.Errorf("node %d left %d out of [0, %d)", i, node.Left, n)
		}
		if node.Right < 0 || node.Right >= n {
			return fmt.Errorf("node %d right %d out of [0, %d)", i, node.Right, n)
		}
		if node.Missing < 0 || node.Missing >= n {
			return fmt.Errorf("node %d missing %d out of [0, %d)", i, node.Missing, n)
		}
	}

	// Iterative DFS to reject cycles reachable from root node 0.
	const (
		unvisited uint8 = 0
		visiting  uint8 = 1
		visited   uint8 = 2
	)
	state := make([]uint8, n)
	type frame struct {
		idx   int
		stage uint8
	}
	stack := make([]frame, 0, n)
	stack = append(stack, frame{idx: 0})
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		idx := top.idx
		switch top.stage {
		case 0:
			switch state[idx] {
			case visited:
				stack = stack[:len(stack)-1]
				continue
			case visiting:
				return fmt.Errorf("cycle detected at node %d", idx)
			}
			state[idx] = visiting
			node := t.Nodes[idx]
			if node.Feature < 0 {
				state[idx] = visited
				stack = stack[:len(stack)-1]
				continue
			}
			top.stage = 1
			stack = append(stack, frame{idx: node.Left})
		case 1:
			top.stage = 2
			stack = append(stack, frame{idx: t.Nodes[idx].Right})
		case 2:
			top.stage = 3
			stack = append(stack, frame{idx: t.Nodes[idx].Missing})
		default:
			state[idx] = visited
			stack = stack[:len(stack)-1]
		}
	}
	return nil
}
