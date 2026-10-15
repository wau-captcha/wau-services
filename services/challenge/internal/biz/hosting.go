package biz

import (
	_ "embed"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"challenge/internal/pkg/envconf"
	"challenge/internal/pkg/iprange"
)

// hostingList holds the address ranges of the cloud and hosting providers: people rarely browse
// from a data centre, and bots usually run in one, so auto_check lets no answer from them through
// without a puzzle. It is written by cmd/hostingranges; refresh it about once a month, from the
// service root, with `go run ./cmd/hostingranges` (or `make hostingranges`), and commit the file.
//
//go:embed hostingranges.txt
var hostingList string

// hostingRanges are CIDR blocks sorted with IPv4 first, none overlapping another, so that the only
// block that can hold an address is the last one starting at or before it.
type hostingRanges []netip.Prefix

// newHostingRanges reads the embedded list and adds extra, the comma-separated blocks of
// extra_hosting_cidrs.
func newHostingRanges(extra string) (hostingRanges, error) {
	var blocks []netip.Prefix
	for line := range strings.Lines(hostingList) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		block, err := netip.ParsePrefix(line)
		if err != nil {
			return nil, fmt.Errorf("hostingranges.txt: %w", err)
		}
		blocks = append(blocks, block)
	}
	for _, text := range envconf.SplitList(extra) {
		block, err := netip.ParsePrefix(text)
		if err != nil {
			return nil, fmt.Errorf("auto_check extra_hosting_cidrs: %q is not a CIDR block", text)
		}
		blocks = append(blocks, block)
	}
	return iprange.Merge(blocks), nil
}

// lookup returns the prefix length of the block holding addr, and whether one does. An IPv4
// address written as IPv6 (::ffff:a.b.c.d) is looked up as IPv4.
func (ranges hostingRanges) lookup(addr netip.Addr) (int, bool) {
	addr = addr.Unmap().WithZone("")
	if !addr.IsValid() {
		return 0, false
	}
	after, _ := slices.BinarySearchFunc(ranges, addr, func(block netip.Prefix, addr netip.Addr) int {
		if block.Addr().Compare(addr) <= 0 {
			return -1
		}
		return 1
	})
	if after == 0 || !ranges[after-1].Contains(addr) {
		return 0, false
	}
	return ranges[after-1].Bits(), true
}

// hostingAddress reports whether addr belongs to a cloud or hosting provider, by the embedded list
// and extra_hosting_cidrs.
func (uc *ChallengeUsecase) hostingAddress(addr netip.Addr) bool {
	_, hosting := uc.auto.hosting.lookup(addr)
	return hosting
}
