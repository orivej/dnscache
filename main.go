package main

import (
	"flag"
	"log"
	"net"
	"time"
)

var (
	flUpstreamIP = IPValueFlag("upstream-ip", net.IP{127, 0, 0, 1},
		"IP address of upstream DNS server")
	flUpstreamPort = flag.Int("upstream-port", 53,
		"UDP port of upstream DNS server")
)

func main() {
	flag.Parse()
	serve()
}

func serve() {
	upstreamUDP := net.UDPAddr{IP: flUpstreamIP.ip, Port: *flUpstreamPort}
	localUDP := net.UDPAddr{Port: 53}

	in, err := net.ListenUDP("udp", &localUDP)
	eexit(err)
	defer eclose(in)

	out, err := net.DialUDP("udp", nil, &upstreamUDP)
	eexit(err)
	defer eclose(out)

	cache := map[string][]byte{}
	inbuf := make([]byte, 65536)
	inid := make([]byte, 2)
	outbuf := make([]byte, 65536)
	counter := uint16(0)

mainLoop:
	for {
		// read incoming
		n, inaddr, err := in.ReadFromUDP(inbuf)
		if eprint(err) || n < 2 {
			continue
		}
		copy(inid, inbuf) // Transaction ID

		// prepare response
		key := string(inbuf[2:])
		var value []byte
		var ok bool
		if value, ok = cache[key]; !ok {
			log.Println("mainLoop (no cache)")
			// prepare for request to timout
		requestLoop:
			for {
				log.Println("requestLoop")
				// query upstream
				counter++
				inbuf[0] = byte(counter >> 8)
				inbuf[1] = byte(counter & 0xff)
				_, err = out.Write(inbuf[:n])
				if eprint(err) {
					continue mainLoop
				}

				// read upstream, dealing with outdated responses
				for {
					log.Println("responseLoop")
					err := out.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
					if eprint(err) {
						continue mainLoop
					}

					n, err = out.Read(outbuf)
					if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
						continue requestLoop
					} else if eprint(err) {
						continue mainLoop
					}
					if inbuf[0] == outbuf[0] && inbuf[1] == outbuf[1] {
						// Transaction IDs match
						break requestLoop
					}
				}
			}

			// save in cache
			value = make([]byte, n)
			copy(value, outbuf)
			cache[key] = value
		}

		// answer incoming
		copy(value, inid) // Transaction ID
		_, err = in.WriteToUDP(value, inaddr)
		if eprint(err) {
			continue
		}
	}
}
