package orderedset

import (
	"math"
	"math/bits"
)

const bitsPerNode = 4
const maxBits = 64
const NotFound = uint64(math.MaxUint64)

type OrderedSet interface {
	Size() int
	LowestUnused() uint64
	Kth(i int) uint64
	LowerBound(value uint64) (uint64, int)
	UpperBound(value uint64) (uint64, int)
	Insert(value uint64)
	Remove(value uint64)
	Contains(value uint64) bool
}

type treeSetNode struct {
	lowestUnused uint64
	size         uint32
	bits         uint16
	offset       uint8
	nextNodes    *treeSetNodes
}

type treeSetNodes [1 << bitsPerNode]*treeSetNode

func (n *treeSetNode) leaf() bool {
	return n.offset == maxBits-bitsPerNode
}

type TreeSet struct {
	root *treeSetNode
}

func NewTreeSet() OrderedSet {
	return &TreeSet{
		root: &treeSetNode{
			offset: maxBits - bitsPerNode,
		},
	}
}

func (set *TreeSet) Size() int {
	return int(set.root.size)
}

func (set *TreeSet) LowestUnused() uint64 {
	return set.root.lowestUnused
}

func (set *TreeSet) Kth(k int) uint64 {
	if int(set.root.size) <= k || k < 0 {
		return NotFound
	}
	curnode := set.root
	var value uint64
	for {
		if curnode.leaf() {
			break
		}
		for i, n := range curnode.nextNodes {
			if n == nil {
				continue
			}
			if int(n.size) <= k {
				k -= int(n.size)
			} else {
				value = (value + uint64(i)) << bitsPerNode
				curnode = n
				break
			}
		}
	}
	for i := uint64(0); i < 1<<bitsPerNode; i++ {
		if (curnode.bits & (1 << i)) > 0 {
			if k == 0 {
				value = value + i
				return value
			} else {
				k--
			}
		}
	}
	return value
}

func (set *TreeSet) LowerBound(value uint64) (uint64, int) {
	sz := set.Size()
	if sz == 0 {
		return NotFound, sz
	}
	left := 0
	right := sz
	for right-left >= 2 {
		mid := (right + left) / 2
		vmid := set.Kth(mid)
		if vmid == value {
			return vmid, mid
		}
		if vmid < value {
			left = mid + 1
		} else {
			right = mid
		}
	}
	for i := left; i <= right; i++ {
		v := set.Kth(i)
		if value <= v && v != NotFound {
			return v, i
		}
	}
	return NotFound, sz
}

func (set *TreeSet) UpperBound(value uint64) (uint64, int) {
	v, i := set.LowerBound(value)
	if v == NotFound || v > value {
		return v, i
	}
	sz := set.Size()
	if i == sz-1 {
		return NotFound, sz
	}
	return set.Kth(i + 1), i + 1
}

func (set *TreeSet) Contains(value uint64) bool {
	if set.root.offset != 0 && 1<<(maxBits-set.root.offset) <= value {
		return false
	}
	curnode := set.root
	for {
		bts := get4BitsFromUint64(curnode.offset, value)
		if curnode.leaf() {
			if (curnode.bits & (1 << bts)) > 0 {
				return true
			} else {
				return false
			}
		}

		if curnode.nextNodes[bts] == nil {
			return false
		}
		curnode = curnode.nextNodes[bts]
	}
}

func (set *TreeSet) Remove(value uint64) {
	if set.root.offset != 0 && 1<<(maxBits-set.root.offset) <= value {
		return
	}
	var path [maxBits / bitsPerNode]*treeSetNode
	pathLength := 0
	curnode := set.root
	for {
		bts := get4BitsFromUint64(curnode.offset, value)
		path[pathLength] = curnode
		pathLength++

		if curnode.leaf() {
			if curnode.bits&(1<<bts) > 0 {
				curnode.bits = curnode.bits & (^(1 << bts))
				for i := pathLength - 1; i >= 0; i-- {
					curnode = path[i]
					curnode.size--
					if curnode.lowestUnused >= value {
						curnode.lowestUnused = value
					}
				}
				removePrevNode := false
				for i := pathLength - 1; i >= 0; i-- {
					curnode = path[i]
					bts = get4BitsFromUint64(curnode.offset, value)

					if removePrevNode {
						curnode.bits = curnode.bits & (^(1 << bts))
						curnode.nextNodes[bts] = nil
						removePrevNode = false
					}
					if curnode.size == 0 {
						removePrevNode = true
					}

				}
				if set.root.size == 0 {
					set.root = &treeSetNode{
						offset: maxBits - bitsPerNode,
					}
				} else {
					for set.root.bits == 1 && set.root.offset < maxBits-bitsPerNode {
						set.root = set.root.nextNodes[0]
					}
				}
			}
			return
		}
		if curnode.nextNodes == nil || curnode.nextNodes[bts] == nil {
			return
		}
		curnode = curnode.nextNodes[bts]
	}
}

func (set *TreeSet) Insert(value uint64) {
	for {
		root := set.root
		if root.offset == 0 || (1<<(maxBits-root.offset)) > value {
			break
		}
		n := treeSetNode{
			bits:         0,
			lowestUnused: set.root.lowestUnused,
			size:         set.root.size,
			offset:       set.root.offset - bitsPerNode,
			nextNodes:    &treeSetNodes{},
		}
		if root.size == 1 {
			n.bits = 1
		}
		n.nextNodes[0] = set.root
		set.root = &n
	}

	var path [maxBits / bitsPerNode]*treeSetNode
	pathLength := 0
	curnode := set.root
	for {
		bts := get4BitsFromUint64(curnode.offset, value)
		path[pathLength] = curnode
		pathLength++

		if curnode.leaf() {
			if curnode.bits&(1<<bts) > 0 {
				return
			} else {
				for i := 0; i < pathLength; i++ {
					path[i].size++
					if path[i].lowestUnused >= value {
						path[i].lowestUnused = math.MaxUint32
					}
				}
			}
			curnode.bits = curnode.bits | (1 << bts)
			break
		}
		curnode.bits = curnode.bits | (1 << bts)

		if curnode.nextNodes[bts] == nil {
			curnode.nextNodes[bts] = &treeSetNode{
				offset:       curnode.offset + bitsPerNode,
				lowestUnused: math.MaxUint32,
			}
			if curnode.nextNodes[bts].leaf() == false {
				curnode.nextNodes[bts].nextNodes = &treeSetNodes{}
			}
		}
		curnode = curnode.nextNodes[bts]
	}

	found := false
	var foundLowestUnused uint64

	for i := pathLength - 1; i >= 0; i-- {
		curnode = path[i]
		if found {
			if curnode.lowestUnused > foundLowestUnused {
				curnode.lowestUnused = foundLowestUnused
			}
			continue
		}
		if curnode.offset != 0 && curnode.size == 1<<(maxBits-curnode.offset) {
			foundLowestUnused = (value>>(maxBits-curnode.offset))<<(maxBits-curnode.offset) + uint64(curnode.size)
			curnode.lowestUnused = foundLowestUnused
			continue
		}
		if curnode.leaf() {
			unsetBit := (^curnode.bits) & (curnode.bits + 1)
			foundLowestUnused = (value>>bitsPerNode)<<bitsPerNode + uint64(bits.TrailingZeros16(unsetBit))
			found = true
			if curnode.lowestUnused > foundLowestUnused {
				curnode.lowestUnused = foundLowestUnused
			}
		} else {
			for j := uint64(0); j < 1<<bitsPerNode; j++ {
				if curnode.nextNodes[j] == nil {
					foundLowestUnused = (value>>(maxBits-curnode.offset))<<(maxBits-curnode.offset) + j*(1<<(maxBits-bitsPerNode-curnode.offset))
					break
				} else if curnode.nextNodes[j].size != 1<<(maxBits-bitsPerNode-curnode.offset) {
					foundLowestUnused = curnode.nextNodes[j].lowestUnused
					break
				}
			}
			found = true
			if curnode.lowestUnused > foundLowestUnused {
				curnode.lowestUnused = foundLowestUnused
			}
		}
	}
}

func get4BitsFromUint64(offset uint8, value uint64) uint64 {
	value = value << offset
	return value >> (maxBits - bitsPerNode)
}
