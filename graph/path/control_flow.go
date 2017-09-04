// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
)

// Dominators returns a dominator tree for all nodes in the flow graph
// g starting from the given root node.
func Dominators(root graph.Node, g graph.Directed) DominatorTree {
	// The algorithm used here is the Lengauer and Tarjan
	// algorithm described in https://doi.org/10.1145%2F357062.357071

	lt := lengauerTarjan{
		parent: make(map[int64]graph.Node),
		pred:   make(map[int64][]graph.Node),
		semi:   make(map[int64]int),
		vertex: make([]graph.Node, 1), // FIXME(kortschak): The current implementation requires non-zero node numbers.
		bucket: make(map[int64]set.Nodes),
		dom:    make(map[int64]graph.Node),

		ancestor: make(map[int64]graph.Node),
		label:    make(map[int64]graph.Node),
	}

	// step 1.
	lt.dfs(g, root)

	for i := lt.n; i >= 2; i-- {
		w := lt.vertex[i]
		wid := w.ID()
		// step 2.
		for _, v := range lt.pred[wid] {
			uid := lt.eval(v).ID()
			if lt.semi[uid] < lt.semi[wid] {
				lt.semi[wid] = lt.semi[uid]
			}
		}

		b, ok := lt.bucket[lt.vertex[lt.semi[wid]].ID()]
		if !ok {
			b = make(set.Nodes)
			lt.bucket[lt.vertex[lt.semi[wid]].ID()] = b
		}
		b.Add(w)
		lt.link(lt.parent[wid], w)

		// step 3.
		for _, v := range lt.bucket[lt.parent[wid].ID()] {
			vid := v.ID()
			lt.bucket[lt.parent[wid].ID()].Remove(v)
			u := lt.eval(v)
			if lt.semi[u.ID()] < lt.semi[vid] {
				lt.dom[vid] = u
			} else {
				lt.dom[vid] = lt.parent[wid]
			}
		}

	}

	// step 4.
	for i := 2; i <= lt.n; i++ {
		w := lt.vertex[i]
		wid := w.ID()
		if lt.dom[wid].ID() != lt.vertex[lt.semi[wid]].ID() {
			lt.dom[wid] = lt.dom[lt.dom[wid].ID()]
		}
	}
	delete(lt.dom, root.ID())

	nodes := make(set.Nodes)
	for _, n := range g.Nodes() {
		nodes.Add(n)
	}
	dominatedBy := make(map[int64][]graph.Node)
	for nid, d := range lt.dom {
		did := d.ID()
		dominatedBy[did] = append(dominatedBy[did], nodes[nid])
	}
	return DominatorTree{root: root, dominatorOf: lt.dom, dominatedBy: dominatedBy}
}

type lengauerTarjan struct {
	// The vertex which is the parent of vertex w
	// in the spanning tree generated by the search.
	parent map[int64]graph.Node

	// The set of vertices v such that (v, w) is an edge
	// of the graph.
	pred map[int64][]graph.Node

	// semi[w] is a number defined as follows:
	// (i)   Before vertex w is numbered, semi[w] = 0, false.
	// (ii)  After w is numbered but before its semidominator
	//       is computed, semi[w] is the number of w.
	// (iii) After the semidominator of w is computed, semi[w]
	//       is the number of the semidominator of w.
	semi map[int64]int

	// vertex[i] is the vertex whose number is i.
	vertex []graph.Node

	// bucket[w] is the set of vertices whose
	// semidominator is w.
	bucket map[int64]set.Nodes

	// dom[w] is vertex defined as follows:
	// (i)  After step 3, if the semidominator of w is its
	//      immediate dominator, then dom[w] is the immediate
	//      dominator of w. Otherwise dom[w] is a vertex v
	//      whose number is smaller than w and whose immediate
	//      dominator is also w's immediate dominator.
	// (ii) After step 4, dom[w] is the immediate dominator of w.
	dom map[int64]graph.Node

	// In general ancestor[v] = 0 only if v is a tree root
	// in the forest; otherwise ancestor[v] is an ancestor
	// of v in the forest.
	ancestor map[int64]graph.Node

	// Initially label[v] is v.
	label map[int64]graph.Node

	n int
}

func (lt *lengauerTarjan) dfs(g graph.Directed, v graph.Node) {
	vid := v.ID()
	lt.n++
	lt.semi[vid] = lt.n
	lt.label[vid] = v
	lt.vertex = append(lt.vertex, v)
	for _, w := range g.From(v) {
		wid := w.ID()
		if _, ok := lt.semi[wid]; !ok {
			lt.parent[wid] = v
			lt.dfs(g, w)
		}
		lt.pred[wid] = append(lt.pred[wid], v)
	}
}

func (lt *lengauerTarjan) compress(v graph.Node) {
	vid := v.ID()
	if _, ok := lt.ancestor[lt.ancestor[vid].ID()]; ok {
		lt.compress(lt.ancestor[vid])
		if lt.semi[lt.label[lt.ancestor[vid].ID()].ID()] < lt.semi[lt.label[vid].ID()] {
			lt.label[vid] = lt.label[lt.ancestor[vid].ID()]
		}
		lt.ancestor[vid] = lt.ancestor[lt.ancestor[vid].ID()]
	}
}

func (lt *lengauerTarjan) eval(v graph.Node) graph.Node {
	vid := v.ID()
	if _, ok := lt.ancestor[vid]; !ok {
		return v
	}
	lt.compress(v)
	return lt.label[vid]
}

func (lt *lengauerTarjan) link(v, w graph.Node) {
	lt.ancestor[w.ID()] = v
}

// DominatorTree is a flow graph dominator tree.
type DominatorTree struct {
	root        graph.Node
	dominatorOf map[int64]graph.Node
	dominatedBy map[int64][]graph.Node
}

// Root returns the root of the tree.
func (d DominatorTree) Root() graph.Node { return d.root }

// DominatorOf returns the immediate dominator of n.
func (d DominatorTree) DominatorOf(n graph.Node) graph.Node {
	return d.dominatorOf[n.ID()]
}

// DominatedBy returns a slice of all nodes immediately dominated by n.
// Elements of the slice are retained by the DominatorTree.
func (d DominatorTree) DominatedBy(n graph.Node) []graph.Node {
	return d.dominatedBy[n.ID()]
}
