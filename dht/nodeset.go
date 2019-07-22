package main

const replicaCount = 3

type nodeSet [replicaCount]nodeFace

func (d nodeSet) set(key string, value string) {
	for _, n := range d {
		n.set(key, value)
	}
}
func (d nodeSet) get(key string) (*string, bool) {
	for _, n := range d {
		res, ok := n.get(key)
		if ok {
			return res, ok
		}
	}
	return nil, false
}
func (d nodeSet) del(key string) {
	for _, n := range d {
		n.del(key)
	}
}

func (d nodeSet) contain(x nodeFace) bool {
	for _, i := range d {
		if i == x {
			return true
		}
	}
	return false
}
