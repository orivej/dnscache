package main

import "net"

var (
	upstreamIP   = net.IP{127, 0, 0, 1}
	upstreamPort = 55
	upstreamUDP  = net.UDPAddr{IP: upstreamIP, Port: upstreamPort}
	localUDP     = net.UDPAddr{Port: 53}
)

func main() {
	serve()
}

func serve() {
	in, err := net.ListenUDP("udp", &localUDP)
	eexit(err)
	defer eclose(in)

	out, err := net.DialUDP("udp", nil, &upstreamUDP)
	eexit(err)
	defer eclose(out)

	cache := map[string][]byte{}

	inbuf := make([]byte, 65536)
	outbuf := make([]byte, 65536)

	for {
		// read incoming
		n, inaddr, err := in.ReadFromUDP(inbuf)
		if eprint(err) || n < 2 {
			continue
		}

		// prepare response
		key := string(inbuf[2:])
		var value []byte
		var ok bool
		if value, ok = cache[key]; ok {
			// update cache
			copy(value, inbuf[:2]) // Transaction ID
		} else {
			// query upstream
			_, err = out.Write(inbuf[:n])
			if eprint(err) {
				continue
			}

			// read upstream
			n, err = out.Read(outbuf)
			if eprint(err) {
				continue
			}

			// save in cache
			value = make([]byte, n)
			copy(value, outbuf)
			cache[key] = value
		}

		// answer incoming
		_, err = in.WriteToUDP(value, inaddr)
		if eprint(err) {
			continue
		}
	}
}
