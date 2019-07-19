package main

func (d *dummyRingInstance) Replicate() {
	for _, v := range d.data {
		logger.Debugf("replicator: test node %016x", v.getID())
		keys := v.allKeys()
		for _, k := range keys {
			ns := d.getSet(k)
			for _, n := range ns {
				_, ok := n.get(k)
				if !ok {
					v, _ := v.get(k) // must be success
					n.set(k, *v)
					logger.Debugf("replicate %s to %016x", k, n.getID())
				}
			}
		}
	}
}
