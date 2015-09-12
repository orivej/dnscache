package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	queryAttempts = 5
	queryTimeout  = 1100 * time.Millisecond
)

var (
	flListen   = flag.String("listen", ":53", "Listen address")
	flUpstream = flag.String("upstream", ":53", "Upstream DNS server")
)

var (
	cache     = map[string]*dns.Msg{}
	cacheLock = sync.Mutex{}
	dnsclient = dns.Client{
		UDPSize:        dns.MaxMsgSize,
		ReadTimeout:    queryTimeout,
		SingleInflight: true,
	}
)

var logFormatExample = `Log format example:
  1EF9┐a19.ru. A IN                      # uncached query
  AAB1┐a19.ru. A IN                      # concurrent query
  152E┐a19.ru. A IN                      # concurrent query
  1EF9└a19.ru. A IN = (A 78.47.223.116)  # answer now cached
  AAB1┴1EF9                              # answer already cached
  152E┴1EF9                              # answer already cached
  2899┐a29.ru. A IN                      # uncached query
  2899·a29.ru. A IN: [1/5] read udp 127.0.0.1:50019->127.0.0.1:53: i/o timeout
    # first attempt of five failed to return an answer
  2899╳a29.ru. A IN: [5/5] read udp 127.0.0.1:50019->127.0.0.1:53: i/o timeout
    # final attempt failed, we sent client servfail
`

func main() {
	log.SetFlags(log.Flags() | log.Lmicroseconds)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Arguments:")
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, logFormatExample)
	}
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
		log.Printf("%04X┐%s\n", req.Id, key)
		for attempt := 1; attempt <= queryAttempts; attempt++ {
			cached, _, err = dnsclient.Exchange(req, *flUpstream)
			if err != nil {
				cacheLock.Lock()
				_, ok = cache[key]
				cacheLock.Unlock()
				if ok {
					// concurrent exchange succeeded
					err = nil
					break
				}
				sep := "·"
				if attempt == queryAttempts {
					sep = "╳"
				}
				log.Printf("%04X%s%s: [%d/%d] %v\n", req.Id, sep, key,
					attempt, queryAttempts, err)
			}
		}
		if err != nil {
			dns.HandleFailed(w, req)
			return
		}
		cacheLock.Lock()
		cached2, ok := cache[key]
		if ok {
			// concurrent exchange has already updated the cache
			cached = cached2
			log.Printf("%04X┴%04X\n", req.Id, cached.Id)
		} else {
			cache[key] = cached
			log.Printf("%04X└%s = %s\n", req.Id, key, answersSummary(cached))
		}
		cacheLock.Unlock()
	}
	resp = *cached
	resp.Id = req.Id
	err = w.WriteMsg(&resp)
	eprint(err)
}
