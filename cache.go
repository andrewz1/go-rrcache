package rrcache

import (
	"time"

	"github.com/miekg/dns"
)

type RRCache interface {
	Get(dns.Question) ([]dns.RR, dns.Question, bool)
	Put([]dns.RR)
}

func NewRRCache(evict time.Duration) RRCache {
	return newRRCache(evict)
}
