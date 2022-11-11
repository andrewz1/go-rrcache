package rrcache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cnameKey struct {
	name  string
	class uint16
}

type cnameVal struct {
	target string
	exp    int64
}

type cnameCache struct {
	m sync.Map
	t *time.Ticker
}

func (cs *cnameCache) load(k cnameKey) (cnameVal, bool) {
	if v, ok := cs.m.Load(k); ok {
		return v.(cnameVal), true
	}
	return cnameVal{}, false
}

func (cs *cnameCache) store(k cnameKey, v cnameVal) {
	cs.m.Store(k, v)
}

func (cs *cnameCache) delete(k cnameKey) {
	cs.m.Delete(k)
}

func (cs *cnameCache) visitAll(f func(cnameKey, cnameVal)) {
	cs.m.Range(func(k, v interface{}) bool {
		f(k.(cnameKey), v.(cnameVal))
		return true
	})
}

func (cs *cnameCache) evict(t int64) {
	cs.visitAll(func(k cnameKey, v cnameVal) {
		if v.ttl(t) == 0 {
			cs.delete(k)
		}
	})
}

func (cs *cnameCache) evictLoop() {
	for t := range cs.t.C {
		cs.evict(t.Unix())
	}
}

func newCnameCache(t time.Duration) *cnameCache {
	cs := &cnameCache{t: time.NewTicker(t)}
	go cs.evictLoop()
	return cs
}

func (cn *cnameVal) ttl(t int64) uint32 {
	if ttl := cn.exp - t; ttl > 0 {
		return uint32(ttl)
	}
	return 0
}

func (ck *cnameKey) hdr(ttl uint32) dns.RR_Header {
	return dns.RR_Header{
		Name:   ck.name,
		Rrtype: dns.TypeCNAME,
		Class:  ck.class,
		Ttl:    ttl,
	}
}

func (cs *cnameCache) get1(k cnameKey) *dns.CNAME {
	cn, ok := cs.load(k)
	if !ok {
		return nil
	}
	ttl := cn.ttl(time.Now().Unix())
	if ttl == 0 {
		return nil
	}
	return &dns.CNAME{
		Hdr:    k.hdr(ttl),
		Target: cn.target,
	}
}

func cnKeyFromQ(q *dns.Question) cnameKey {
	return cnameKey{
		name:  dns.CanonicalName(q.Name),
		class: q.Qclass,
	}
}

func cnKeyFromCname(cn *dns.CNAME) cnameKey {
	return cnameKey{
		name:  dns.CanonicalName(cn.Target),
		class: cn.Hdr.Class,
	}
}

func cnKeyFromHdr(h *dns.RR_Header) cnameKey {
	return cnameKey{
		name:  dns.CanonicalName(h.Name),
		class: h.Class,
	}
}

func cnKeyFromAny(v interface{}) cnameKey {
	switch vv := v.(type) {
	case *dns.Question:
		return cnKeyFromQ(vv)
	case *dns.CNAME:
		return cnKeyFromCname(vv)
	default:
		panic("invalid type")
	}
}

func (cs *cnameCache) get(q dns.Question) ([]dns.RR, dns.Question) {
	rv := make([]dns.RR, 0, 10)
	var src interface{} = &q
	for {
		k := cnKeyFromAny(src)
		cn := cs.get1(k)
		if cn == nil {
			break
		}
		rv = append(rv, cn)
		src = cn
	}
	l := len(rv)
	if l == 0 {
		return nil, q
	}
	cn := rv[l-1].(*dns.CNAME)
	q.Name = dns.CanonicalName(cn.Target)
	return rv, q
}

func getExp(h *dns.RR_Header) int64 {
	return time.Now().Unix() + int64(h.Ttl)
}

func (cs *cnameCache) put1(cn *dns.CNAME) {
	h := cn.Header()
	k := cnKeyFromHdr(h)
	cs.store(k, cnameVal{target: dns.CanonicalName(cn.Target), exp: getExp(h)})
}
