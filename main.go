package main

import (
	"flag"
	"log"
	"sync"

	"github.com/miekg/dns"
)

var (
	flUpstream = flag.String("upstream", "127.0.0.1:53",
		"Upstream DNS server")
)

var (
	cache     = map[string]*dns.Msg{}
	cacheLock = sync.Mutex{}
	dnsclient = dns.Client{UDPSize: dns.MaxMsgSize, SingleInflight: true}
)

func main() {
	flag.Parse()

	server := dns.Server{Net: "udp"}
	dns.HandleFunc(".", serve)
	err := server.ListenAndServe()
	eexit(err)
}

func msgKey(m *dns.Msg) string {
	t := ""
	if t1, ok := dns.TypeToString[m.Question[0].Qtype]; ok {
		t = t1
	}
	cl := ""
	if cl1, ok := dns.ClassToString[m.Question[0].Qclass]; ok {
		cl = cl1
	}
	return m.Question[0].Name + " " + t + " " + cl
}

func serve(w dns.ResponseWriter, req *dns.Msg) {
	var resp *dns.Msg
	var err error
	key := msgKey(req)
	cacheLock.Lock()
	resp, ok := cache[key]
	cacheLock.Unlock()
	if ok {
		// SingleInflight ensures that resp will not be changed until the end of serve
		resp.Id = req.Id
	} else {
		resp, _, err = dnsclient.Exchange(req, *flUpstream)
		if err != nil {
			log.Printf("Can not handle %v: %v\n", key, err)
			dns.HandleFailed(w, req)
			return
		}
		cacheLock.Lock()
		cache[key] = resp
		cacheLock.Unlock()
	}
	err = w.WriteMsg(resp)
	eprint(err)
}
