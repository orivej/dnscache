package main

import (
	"errors"
	"flag"
	"net"
)

type IPValue struct {
	ip net.IP
}

func (ipv *IPValue) Set(s string) error {
	ipv.ip = net.ParseIP(s)
	if ipv.ip == nil {
		return errors.New("not an IP")
	}
	return nil
}

func (ipv *IPValue) String() string {
	return ipv.ip.String()
}

func IPValueFlag(name string, value net.IP, usage string) *IPValue {
	ipv := IPValue{value}
	flag.Var(&ipv, name, usage)
	return &ipv
}
