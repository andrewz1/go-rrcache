package rrcache

import (
	"time"

	"github.com/miekg/dns"
)

const (
	minDnsEvictInt = 1 * time.Second
)

type rrCache struct {
	cs *cnameCache
	ds *dataCache
}

func newRRCache(evict time.Duration) *rrCache {
	if evict < minDnsEvictInt {
		evict = minDnsEvictInt
	}
	return &rrCache{
		cs: newCnameCache(evict),
		ds: newDataCache(evict),
	}
}

func (ds *rrCache) Get(q dns.Question) ([]dns.RR, dns.Question, bool) {
	rv := make([]dns.RR, 0, 10)
	// check CNAME cache
	var cns []dns.RR
	cns, q = ds.cs.get(q)
	rv = append(rv, cns...)
	// check RR cache
	var rrs []dns.RR
	rrs = ds.ds.get(q)
	rv = append(rv, rrs...)
	return rv, q, len(rrs) == 0
}

func (ds *rrCache) Put(rrs []dns.RR) {
	for _, rr := range rrs {
		if v, ok := rr.(*dns.CNAME); ok {
			ds.cs.put1(v)
		} else {
			ds.ds.put1(rr)
		}
	}
}
