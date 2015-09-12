package main

import (
	"flag"
	"log"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

var (
	flListen   = flag.String("listen", ":53", "Listen address")
	flUpstream = flag.String("upstream", ":53", "Upstream DNS server")
)

var (
	cache     = map[string]*dns.Msg{}
	cacheLock = sync.Mutex{}
	dnsclient = dns.Client{UDPSize: dns.MaxMsgSize, SingleInflight: true}
)

func main() {
	log.SetFlags(log.Flags() | log.Lmicroseconds)
	flag.Parse()

	dns.HandleFunc(".", serve)
	server := dns.Server{Addr: *flListen, Net: "udp"}
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

func recordSummary(r dns.RR) string {
	s := r.String()
	parts := strings.Split(s, "\t")
	nparts := len(parts)
	if nparts < 2 {
		return s
	}
	return "(" + strings.Join(parts[nparts-2:], " ") + ")"
}

func answersSummary(m *dns.Msg) string {
	var answers []string
	if m.Answer != nil {
		for _, rr := range m.Answer {
			answers = append(answers, recordSummary(rr))
		}
	}
	return strings.Join(answers, " ")
}

func serve(w dns.ResponseWriter, req *dns.Msg) {
	var resp dns.Msg
	var err error
	key := msgKey(req)
	cacheLock.Lock()
	cached, ok := cache[key]
	// cached value will not change until return because of SingleInflight
	cacheLock.Unlock()
	if !ok {
		log.Printf(`%X┐%s`, req.Id, key)
		cached, _, err = dnsclient.Exchange(req, *flUpstream)
		if err != nil {
			log.Printf(`%X╳%s: %v`, req.Id, key, err)
			dns.HandleFailed(w, req)
			return
		}
		log.Printf(`%X└%s = %s`, req.Id, key, answersSummary(cached))
		cacheLock.Lock()
		cache[key] = cached
		cacheLock.Unlock()
	}
	resp = *cached
	resp.Id = req.Id
	err = w.WriteMsg(&resp)
	eprint(err)
}
