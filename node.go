package gkvlite

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
)

// A persistable node.
type node struct {
	numNodes, numBytes uint64
	item               itemLoc
	left, right        nodeLoc
	next               *node // For free-list tracking.
}

func (n *node) Evict() *Item {
	if !n.item.Loc().isEmpty() {
		i := n.item.Item()
		if i != nil && n.item.casItem(i, nil) {
			return i
		}
	}
	return nil
}

// A persistable node and its persistence location.
type nodeLoc struct {
	m sync.Mutex

	loc  *ploc    // *ploc - can be nil if node is dirty (not yet persisted).
	node *node    // *node - can be nil if node is not fetched into memory yet.
	next *nodeLoc // For free-list tracking.
}

var empty_nodeLoc = &nodeLoc{} // Sentinel.

func (nloc *nodeLoc) Loc() *ploc {
	nloc.m.Lock()
	defer nloc.m.Unlock()
	return nloc.loc
}

func (nloc *nodeLoc) setLoc(n *ploc) {
	nloc.m.Lock()
	defer nloc.m.Unlock()
	nloc.loc = n
}

func (nloc *nodeLoc) Node() *node {
	nloc.m.Lock()
	defer nloc.m.Unlock()
	return nloc.node
}

func (nloc *nodeLoc) setNode(n *node) {
	nloc.m.Lock()
	defer nloc.m.Unlock()
	nloc.node = n
}

func (nloc *nodeLoc) Copy(src *nodeLoc) *nodeLoc {
	if src == nil {
		return nloc.Copy(empty_nodeLoc)
	}
	newloc := src.Loc()
	newnode := src.Node()

	nloc.m.Lock()
	defer nloc.m.Unlock()
	nloc.loc = newloc
	nloc.node = newnode
	return nloc
}

func (nloc *nodeLoc) isEmpty() bool {
	return nloc == nil || (nloc.Loc().isEmpty() && nloc.Node() == nil)
}

func (nloc *nodeLoc) write(o *Store) error {
	if nloc != nil && nloc.Loc().isEmpty() {
		node := nloc.Node()
		if node == nil {
			return nil
		}
		offset := atomic.LoadInt64(&o.size)
		length := ploc_length + ploc_length + ploc_length + 8 + 8
		b := make([]byte, length)
		pos := 0
		pos = node.item.Loc().write(b, pos)
		pos = node.left.Loc().write(b, pos)
		pos = node.right.Loc().write(b, pos)
		binary.BigEndian.PutUint64(b[pos:pos+8], node.numNodes)
		pos += 8
		binary.BigEndian.PutUint64(b[pos:pos+8], node.numBytes)
		pos += 8
		if pos != length {
			return fmt.Errorf("nodeLoc.write() pos: %v didn't match length: %v",
				pos, length)
		}
		if _, err := o.file.WriteAt(b, offset); err != nil {
			return err
		}
		atomic.StoreInt64(&o.size, offset+int64(length))
		nloc.setLoc(&ploc{Offset: offset, Length: uint32(length)})
	}
	return nil
}

func (nloc *nodeLoc) read(o *Store) (n *node, err error) {
	if nloc == nil {
		return nil, nil
	}
	n = nloc.Node()
	if n != nil {
		return n, nil
	}
	loc := nloc.Loc()
	if loc.isEmpty() {
		return nil, nil
	}
	if loc.Length != uint32(ploc_length+ploc_length+ploc_length+8+8) {
		return nil, fmt.Errorf("unexpected node loc.Length: %v != %v",
			loc.Length, ploc_length+ploc_length+ploc_length+8+8)
	}
	b := make([]byte, loc.Length)
	if _, err := o.file.ReadAt(b, loc.Offset); err != nil {
		return nil, err
	}
	pos := 0
	atomic.AddUint64(&o.nodeAllocs, 1)
	n = &node{}
	var p *ploc
	p = &ploc{}
	p, pos = p.read(b, pos)
	n.item.loc = p
	p = &ploc{}
	p, pos = p.read(b, pos)
	n.left.loc = p
	p = &ploc{}
	p, pos = p.read(b, pos)
	n.right.loc = p
	n.numNodes = binary.BigEndian.Uint64(b[pos : pos+8])
	pos += 8
	n.numBytes = binary.BigEndian.Uint64(b[pos : pos+8])
	pos += 8
	if pos != len(b) {
		return nil, fmt.Errorf("nodeLoc.read() pos: %v didn't match length: %v",
			pos, len(b))
	}
	nloc.setNode(n)
	return n, nil
}

func numInfo(o *Store, left *nodeLoc, right *nodeLoc) (
	leftNum uint64, leftBytes uint64, rightNum uint64, rightBytes uint64, err error) {
	leftNode, err := left.read(o)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	rightNode, err := right.read(o)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if !left.isEmpty() && leftNode != nil {
		leftNum = leftNode.numNodes
		leftBytes = leftNode.numBytes
	}
	if !right.isEmpty() && rightNode != nil {
		rightNum = rightNode.numNodes
		rightBytes = rightNode.numBytes
	}
	return leftNum, leftBytes, rightNum, rightBytes, nil
}

func dump(o *Store, n *nodeLoc, level int) {
	if n.isEmpty() {
		return
	}
	nNode, _ := n.read(o)
	dump(o, &nNode.left, level+1)
	dumpIndent(level)
	k := "<evicted>"
	if nNode.item.Item() != nil {
		k = string(nNode.item.Item().Key)
	}
	fmt.Printf("%p - %v\n", nNode, k)
	dump(o, &nNode.right, level+1)
}

func dumpIndent(level int) {
	for i := 0; i < level; i++ {
		fmt.Print(" ")
	}
}
