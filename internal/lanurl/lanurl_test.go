package lanurl

import (
	"net"
	"testing"
)

// The rules are all about machines nobody testing this has: three
// interfaces, a VPN, a laptop with the cable out. That's why For takes
// the interface list rather than reading it.

func iface(name string, ips ...string) Interface {
	parsed := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		parsed = append(parsed, net.ParseIP(ip))
	}
	return Interface{Name: name, IPs: parsed}
}

func urls(candidates []Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.URL)
	}
	return out
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestFor_ListsEveryInterfaceRatherThanGuessing(t *testing.T) {
	got := For(":8080", []Interface{
		iface("Ethernet", "192.168.1.23"),
		iface("Wi-Fi", "192.168.1.24"),
	})

	want := []string{"http://192.168.1.23:8080", "http://192.168.1.24:8080"}
	if !equal(urls(got), want) {
		t.Fatalf("urls = %v, want %v", urls(got), want)
	}
	// The name travels with the address: on a machine with three of
	// these, the Host is the only one who knows which network the table
	// is on.
	if got[0].Interface != "Ethernet" || got[1].Interface != "Wi-Fi" {
		t.Fatalf("interfaces = %q/%q, want Ethernet/Wi-Fi", got[0].Interface, got[1].Interface)
	}
}

// A private address is the one a table on a home network can actually
// use, so it goes first — the VPN address below it is real, reachable
// by somebody, and almost never the answer.
func TestFor_PutsPrivateAddressesFirst(t *testing.T) {
	got := urls(For(":8080", []Interface{
		iface("VPN", "100.64.3.9"),
		iface("Wi-Fi", "192.168.0.10"),
	}))

	want := []string{"http://192.168.0.10:8080", "http://100.64.3.9:8080"}
	if !equal(got, want) {
		t.Fatalf("urls = %v, want the private address first: %v", got, want)
	}
}

// Loopback works for exactly the person who doesn't need it; link-local
// means DHCP failed; IPv6 is not something anyone types into a phone.
func TestFor_SkipsAddressesNobodyCanUse(t *testing.T) {
	got := urls(For(":8080", []Interface{
		iface("lo", "127.0.0.1"),
		iface("Ethernet", "169.254.14.2"),
		iface("Wi-Fi", "fe80::1", "2001:db8::1", "192.168.1.5"),
	}))

	if want := []string{"http://192.168.1.5:8080"}; !equal(got, want) {
		t.Fatalf("urls = %v, want %v", got, want)
	}
}

// The one that would otherwise send a table chasing an address that was
// never listening: a server bound to one interface is reachable there
// and nowhere else, whatever the machine's other addresses are.
func TestFor_ASpecificBindIsAnsweredWithItself(t *testing.T) {
	got := urls(For("127.0.0.1:8080", []Interface{iface("Wi-Fi", "192.168.1.5")}))

	if want := []string{"http://127.0.0.1:8080"}; !equal(got, want) {
		t.Fatalf("urls = %v, want %v", got, want)
	}
}

func TestFor_TreatsEveryWildcardSpellingAsAllInterfaces(t *testing.T) {
	ifaces := []Interface{iface("Wi-Fi", "192.168.1.5")}

	for _, addr := range []string{":8080", "0.0.0.0:8080", "[::]:8080"} {
		got := urls(For(addr, ifaces))
		if want := []string{"http://192.168.1.5:8080"}; !equal(got, want) {
			t.Fatalf("%s: urls = %v, want %v", addr, got, want)
		}
	}
}

// A laptop with the cable out and Wi-Fi off has nothing to print, and
// nothing is the honest answer — the caller says so in words rather
// than printing a URL that goes nowhere.
func TestFor_NoUsableAddressesIsEmpty(t *testing.T) {
	if got := For(":8080", []Interface{iface("lo", "127.0.0.1")}); len(got) != 0 {
		t.Fatalf("urls = %v, want none", urls(got))
	}
}

func TestFor_UnparseableAddressIsEmptyRatherThanAPanic(t *testing.T) {
	if got := For("not-an-address", []Interface{iface("Wi-Fi", "192.168.1.5")}); got != nil {
		t.Fatalf("urls = %v, want none", urls(got))
	}
}

// The port travels from the flag, since a Host who moved the server off
// 8080 is exactly the one who can't guess the URL.
func TestFor_CarriesTheListeningPort(t *testing.T) {
	got := urls(For(":9999", []Interface{iface("Wi-Fi", "192.168.1.5")}))

	if want := []string{"http://192.168.1.5:9999"}; !equal(got, want) {
		t.Fatalf("urls = %v, want %v", got, want)
	}
}

// Reads whatever this machine has. Asserts only what must be true
// everywhere — it runs on a laptop, a CI container and a build agent,
// and any claim about *which* addresses exist would be a claim about
// one of them.
func TestInterfaces_ReadsTheRealMachineWithoutFailing(t *testing.T) {
	ifaces, err := Interfaces()
	if err != nil {
		t.Fatalf("Interfaces: %v", err)
	}
	for _, iface := range ifaces {
		if iface.Name == "" {
			t.Errorf("interface with no name: %+v", iface)
		}
		if len(iface.IPs) == 0 {
			t.Errorf("%s: kept with no addresses", iface.Name)
		}
	}
}
