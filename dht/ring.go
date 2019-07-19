package main

import (
	"fmt"
	"math/rand"
	"sort"
)

type ringFace interface {
	nodeFace
}

type dummyRingInstance struct {
	hasher hashFace
	data   map[uint64]nodeFace
}

func (d *dummyRingInstance) getSet(key string) nodeSet {
	var res nodeSet
	var nodeIdx = 0

	h := d.hasher.hash(key)
	keys := make([]uint64, 0, len(d.data))
	for k := range d.data {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		if k > h {
			res[nodeIdx] = d.data[k]
			nodeIdx++
		}
		if nodeIdx >= replicaCount {
			break
		}
	}
	for i := 0; i < replicaCount-nodeIdx; i++ {
		res[nodeIdx] = d.data[keys[i]]
		nodeIdx++
	}
	var debugLog string
	for i := 0; i < replicaCount; i++ {
		if len(debugLog) > 0 {
			debugLog += ", "
		}
		debugLog += fmt.Sprintf("%016x", res[i].getID())
	}
	logger.Debugf("use nodes [%s] for key %s (%x)", debugLog, key, h)
	return res
}

func (d *dummyRingInstance) set(key string, value string) {
	d.getSet(key).set(key, value)
}
func (d *dummyRingInstance) get(key string) (*string, bool) {
	return d.getSet(key).get(key)
}
func (d *dummyRingInstance) del(key string) {
	d.getSet(key).del(key)
}
func (d *dummyRingInstance) clear() {
	for _, n := range d.data {
		n.clear()
	}
}
func (d *dummyRingInstance) getID() uint64 {
	return 0
}
func (d *dummyRingInstance) dropNode(id uint64) {
	delete(d.data, id)
}

func newDummyRing(h hashFace, size int) *dummyRingInstance {
	if size < replicaCount {
		logger.Fatalf("ring size too small")
	}
	x := &dummyRingInstance{
		hasher: h,
		data:   make(map[uint64]nodeFace),
	}
	for i := 0; i < size; i++ {
		id := rand.Uint64()
		el := newDummyNode(id)
		logger.Debugf("created instance %016x", id)
		x.data[id] = el // FIXME: ensure uniq ids
	}
	return x
}
