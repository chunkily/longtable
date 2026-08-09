// Package lanurl works out which addresses a Host can hand to the
// people at their table.
//
// The alternative is the Host going hunting through OS network settings
// for their own IP, which is the first thing anyone has to do to use
// Longtable at all — everyone else joins over the LAN, and nobody can
// join anything until somebody reads out an address.
package lanurl

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// Interface is one network interface and the addresses it holds, as
// this package needs them. A plain struct rather than *net.Interface so
// the candidate rules can be tested against machines nobody has —
// three interfaces, a VPN, no network at all — instead of only against
// whatever the machine running the tests happens to be plugged into.
type Interface struct {
	Name string
	IPs  []net.IP
}

// Candidate is one address worth printing, and the interface it came
// from. The name is carried because a Host with Wi-Fi, Ethernet and a
// VPN gets three of these and is the only one who knows which their
// players are on — see For.
type Candidate struct {
	Interface string
	URL       string
}

// For returns the addresses worth showing for a server listening on
// listenAddr.
//
// **A specific listen address is answered with itself.** Someone who
// started the server on `-addr 127.0.0.1:8080` is not reachable over
// the LAN at all, and printing the machine's Wi-Fi address there would
// be a lie that costs a table twenty minutes. Only a wildcard bind —
// `:8080`, `0.0.0.0:8080`, `[::]:8080` — means "every interface", and
// only then is enumerating them the right answer.
//
// Nothing here guesses which interface is the one to use. A machine
// with Wi-Fi, Ethernet and a VPN gets three lines and the Host picks;
// the alternative is picking for them and being wrong on exactly the
// setups where being wrong is hardest to debug.
func For(listenAddr string, ifaces []Interface) []Candidate {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil
	}

	if host != "" && host != "0.0.0.0" && host != "::" {
		return []Candidate{{URL: urlFor(host, port)}}
	}

	var out []Candidate
	for _, iface := range ifaces {
		for _, ip := range iface.IPs {
			if !usable(ip) {
				continue
			}
			out = append(out, Candidate{Interface: iface.Name, URL: urlFor(ip.String(), port)})
		}
	}

	// Private addresses first, then by interface name, so the same
	// machine prints the same order every time and the address most
	// likely to be the one — a home LAN's 192.168.x.x — is at the top.
	sort.SliceStable(out, func(i, j int) bool {
		iPrivate, jPrivate := isPrivate(out[i].URL), isPrivate(out[j].URL)
		if iPrivate != jPrivate {
			return iPrivate
		}
		if out[i].Interface != out[j].Interface {
			return out[i].Interface < out[j].Interface
		}
		return out[i].URL < out[j].URL
	})
	return out
}

// usable reports whether an address is one a person could type into a
// phone on the same network.
//
// IPv4 only, deliberately. Longtable's deployment story is a handful of
// people on a home LAN, where the address gets read out loud or typed
// with a thumb; an IPv6 address is neither of those things, and a
// machine that has one has an IPv4 address too. Loopback is skipped
// because it is the address that works for exactly the person who
// doesn't need it, and link-local (169.254.x.x) because it means DHCP
// failed — it is a symptom, not somewhere to send a table.
func usable(ip net.IP) bool {
	return ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
}

func isPrivate(url string) bool {
	host := strings.TrimPrefix(url, "http://")
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

func urlFor(host, port string) string {
	return fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
}

// Interfaces reads the machine's own network interfaces, skipping the
// ones that are down. A failure to read them is not worth failing a
// server start over: the worst outcome is a Host who has to look their
// address up the old way.
func Interfaces() ([]Interface, error) {
	system, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var out []Interface
	for _, iface := range system {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ips []net.IP
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				ips = append(ips, ipNet.IP)
			}
		}
		if len(ips) > 0 {
			out = append(out, Interface{Name: iface.Name, IPs: ips})
		}
	}
	return out, nil
}
