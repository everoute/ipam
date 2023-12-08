package utils

import (
	"encoding/binary"
	"math"
	"net"
)

func Ipv4ToUint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}

func Uint32ToIpv4(ip uint32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, ip)
	return net.IPv4(b[0], b[1], b[2], b[3])
}

func FirstIP(ipNet *net.IPNet) net.IP {
	return ipNet.IP
}

func LastIP(ipNet *net.IPNet) net.IP {
	ones, bits := ipNet.Mask.Size()
	uintIP := Ipv4ToUint32(ipNet.IP)
	add := uint32(math.Pow(2, float64(bits-ones))) - 1
	return Uint32ToIpv4(uintIP + add)
}
