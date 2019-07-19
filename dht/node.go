package main

type keyList []string

type nodeFace interface {
	set(key string, value string)
	get(key string) (*string, bool)
	del(key string)
	clear()
	getID() uint64
	allKeys() keyList
}

type dummyNode struct {
	id   uint64
	data map[string]*string
}

func (d *dummyNode) set(key string, value string) {
	d.data[key] = &value
}
func (d *dummyNode) get(key string) (*string, bool) {
	i, ok := d.data[key]
	return i, ok
}
func (d *dummyNode) del(key string) {
	delete(d.data, key)
}
func (d *dummyNode) clear() {
	d.data = make(map[string]*string)
}
func (d *dummyNode) getID() uint64 {
	return d.id
}
func (d *dummyNode) allKeys() keyList {
	var kl = make(keyList, 0, len(d.data))
	for k := range d.data {
		kl = append(kl, k)
	}
	return kl
}

func newDummyNode(id uint64) nodeFace {
	return &dummyNode{
		id:   id,
		data: make(map[string]*string),
	}
}
