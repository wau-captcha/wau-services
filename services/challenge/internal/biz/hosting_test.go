package biz

import (
	"math/rand/v2"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"challenge/internal/conf"
)

func TestHostingAddress(t *testing.T) {
	f := newFixture(nil, func(c *conf.AutoCheck) { c.ExtraHostingCidrs = "198.51.100.0/24, 2001:db8:1::/48" })
	for text, want := range map[string]bool{
		"3.16.0.10":              true,  // AWS EC2, us-east-2 (3.16.0.0/14)
		"::ffff:3.16.0.10":       true,  // the same, written as IPv6
		"2600:1f10:4000::1":      true,  // AWS EC2 IPv6, us-east-1 (2600:1f10:4000::/36)
		"34.80.0.1":              true,  // Google Cloud, asia-east1
		"150.136.0.1":            true,  // Oracle Cloud, Ashburn
		"159.89.0.1":             true,  // DigitalOcean
		"172.104.8.1":            true,  // Linode
		"2600:3c00::1":           true,  // Linode IPv6
		"198.51.100.7":           true,  // extra_hosting_cidrs
		"2001:db8:1::5":          true,  // extra_hosting_cidrs, IPv6
		"203.0.113.4":            false, // a home address stands in for any other
		"113.161.0.1":            false, // VNPT, a Vietnamese carrier
		"2001:db8:2::5":          false, // next to an extra range
		"1.1.1.1":                false, // Cloudflare: CDN and proxy ranges are left out
		"8.8.8.8":                false, // Google's own services, not Google Cloud
		"0.0.0.0":                false,
		"::":                     false,
		"fe80::1%eth0":           false,
		"2600:1f10:4000::1%eth0": true, // the zone does not count
		"255.255.255.255":        false,
	} {
		if got := f.usecase.hostingAddress(netip.MustParseAddr(text)); got != want {
			t.Errorf("%s: got %v, want %v", text, got, want)
		}
	}
	if f.usecase.hostingAddress(netip.Addr{}) {
		t.Error("the zero address is a hosting one")
	}
	// The extra ranges belong to the settings that name them.
	if newFixture(nil).usecase.hostingAddress(netip.MustParseAddr("198.51.100.7")) {
		t.Error("198.51.100.7 is a hosting address without extra_hosting_cidrs")
	}
	// The prefix length goes into the log line in place of the address.
	if bits, ok := f.usecase.auto.hosting.lookup(netip.MustParseAddr("3.16.0.10")); !ok || bits != 12 {
		t.Errorf("3.16.0.10: /%d %v, want /12 true", bits, ok)
	}
}

// The list is written merged and sorted, so reading it changes nothing, and every line is a block.
func TestHostingListIsMerged(t *testing.T) {
	ranges, err := newHostingRanges("")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.Lines(hostingList) {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	var blocks []string
	for _, block := range ranges {
		blocks = append(blocks, block.String())
	}
	if len(blocks) < 1000 || !slices.Equal(blocks, lines) {
		t.Fatalf("%d ranges read from %d lines", len(blocks), len(lines))
	}
}

func BenchmarkHostingAddress(b *testing.B) {
	f := newFixture(nil)
	rng := rand.New(rand.NewPCG(1, 2))
	addrs := make([]netip.Addr, 4096)
	for i := range addrs {
		if i%2 == 0 {
			var v4 [4]byte
			for j := range v4 {
				v4[j] = byte(rng.IntN(256))
			}
			addrs[i] = netip.AddrFrom4(v4)
			continue
		}
		var v6 [16]byte
		v6[0], v6[1] = 0x26, 0x00
		for j := 2; j < 16; j++ {
			v6[j] = byte(rng.IntN(256))
		}
		addrs[i] = netip.AddrFrom16(v6)
	}
	for i := 0; b.Loop(); i++ {
		f.usecase.hostingAddress(addrs[i%len(addrs)])
	}
	b.ReportMetric(float64(len(f.usecase.auto.hosting)), "ranges")
}
