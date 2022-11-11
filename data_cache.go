package rrcache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type dataKey struct {
	name  string
	rtype uint16
	class uint16
}

type dataOne struct {
	rr  dns.RR
	exp int64
}

type dataVal []dataOne

type dataCache struct {
	m sync.Map
	t *time.Ticker
}

func (ds *dataCache) load(k dataKey) (dataVal, bool) {
	if v, ok := ds.m.Load(k); ok {
		return v.(dataVal), true
	}
	return nil, false
}

func (ds *dataCache) store(k dataKey, v dataVal) {
	if len(v) > 0 {
		ds.m.Store(k, v)
	} else {
		ds.m.Delete(k)
	}
}

func (ds *dataCache) delete(k dataKey) {
	ds.m.Delete(k)
}

func (ds *dataCache) visitAll(f func(dataKey, dataVal)) {
	ds.m.Range(func(k, v interface{}) bool {
		f(k.(dataKey), v.(dataVal))
		return true
	})
}

func (ds *dataCache) add(k dataKey, v dataOne) {
	dv, _ := ds.load(k)
	dv = append(dv, v)
	ds.store(k, dv)
}

func (do *dataOne) ttl(t int64) uint32 {
	if ttl := do.exp - t; ttl > 0 {
		return uint32(ttl)
	}
	return 0
}

func (ds *dataCache) evict(t int64) {
	ds.visitAll(func(k dataKey, v dataVal) {
		dv := make(dataVal, 0, len(v))
		for _, do := range v {
			if do.ttl(t) > 0 {
				dv = append(dv, do)
			}
		}
		ds.store(k, dv)
	})
}

func (ds *dataCache) evictLoop() {
	for t := range ds.t.C {
		ds.evict(t.Unix())
	}
}

func newDataCache(t time.Duration) *dataCache {
	ds := &dataCache{t: time.NewTicker(t)}
	go ds.evictLoop()
	return ds
}

func dKeyFromQ(q *dns.Question) dataKey {
	return dataKey{
		name:  dns.CanonicalName(q.Name),
		rtype: q.Qtype,
		class: q.Qclass,
	}
}

func dKeyFromHdr(h *dns.RR_Header) dataKey {
	return dataKey{
		name:  dns.CanonicalName(h.Name),
		rtype: h.Rrtype,
		class: h.Class,
	}
}

func (ds *dataCache) get(q dns.Question) []dns.RR {
	k := dKeyFromQ(&q)
	dv, ok := ds.load(k)
	if !ok {
		return nil
	}
	rv := make([]dns.RR, 0, len(dv))
	t := time.Now().Unix()
	for _, v := range dv {
		ttl := v.ttl(t)
		if ttl == 0 {
			continue
		}
		rr := dns.Copy(v.rr)
		rr.Header().Ttl = ttl
		rv = append(rv, rr)
	}
	if len(rv) > 0 {
		return rv
	}
	return nil
}

func (ds *dataCache) put1(rr dns.RR) {
	h := rr.Header()
	if h.Name == "." || h.Rrtype == dns.TypeOPT || h.Ttl == 0 {
		return
	}
	k := dKeyFromHdr(h)
	ds.add(k, dataOne{rr: rr, exp: getExp(h)})
}
