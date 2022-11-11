package rrcache

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func makeMsg(q dns.Question, rrs []dns.RR) *dns.Msg {
	return &dns.Msg{
		Question: []dns.Question{q},
		Answer:   rrs,
	}
}

func TestReq(t *testing.T) {
	ds := NewRRCache(0)

	cl := dns.Client{
		Net:            "tcp-tls",
		SingleInflight: true,
	}
	cn, err := cl.Dial("1.1.1.1:853")
	if err != nil {
		t.Fatal(err)
	}
	defer cn.Close()
	m := &dns.Msg{}
	m.SetQuestion(dns.CanonicalName("captive.apple.com"), dns.TypeA)
	t.Logf("%s\n", m)

	rrs, q, do := ds.Get(m.Question[0])
	t.Logf("\ndo:%v\n", do)
	t.Logf("%s\n", makeMsg(q, rrs))

	r, rtt, err := cl.ExchangeWithConn(m, cn)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\nrtt: %v\n%+v\n", rtt, r)

	ds.Put(r.Answer)

	time.Sleep(2 * time.Second)

	rrs, q, do = ds.Get(m.Question[0])
	t.Logf("\ndo:%v\n", do)
	t.Logf("%s\n", makeMsg(q, rrs))

}
