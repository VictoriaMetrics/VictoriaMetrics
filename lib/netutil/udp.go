package netutil

import "flag"

var enableUDP6 = flag.Bool("enableUDP6", false, "Whether to enable IPv6 for listening. By default only IPv4 UDP is used")

// GetUDPNetwork returns current udp network.
func GetUDPNetwork() string {
	if *enableUDP6 {
		// Enable both udp4 and udp6
		return "udp"
	}
	return "udp4"
}
