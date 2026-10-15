// Package iprange merges CIDR blocks, for the list of hosting ranges (biz/hosting.go) and the
// command that writes it (cmd/hostingranges).
package iprange

import (
	"net/netip"
	"slices"
)

// span is a run of addresses of one family, first to last included.
type span struct{ first, last netip.Addr }

// Merge returns the fewest CIDR blocks that cover exactly the addresses of prefixes, sorted with
// IPv4 first: overlapping and adjacent blocks are joined, so no two returned blocks overlap and a
// lookup can binary-search them. Invalid prefixes are left out.
func Merge(prefixes []netip.Prefix) []netip.Prefix {
	spans := make([]span, 0, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() {
			continue
		}
		prefix = prefix.Masked()
		spans = append(spans, span{first: prefix.Addr(), last: Last(prefix)})
	}
	slices.SortFunc(spans, func(a, b span) int { return a.first.Compare(b.first) })

	var merged []span
	for _, next := range spans {
		if n := len(merged); n > 0 && merged[n-1].first.Is4() == next.first.Is4() {
			current := &merged[n-1]
			// The run after current starts at current.last.Next(), invalid past the last address.
			after := current.last.Next()
			if !after.IsValid() || next.first.Compare(after) <= 0 {
				if next.last.Compare(current.last) > 0 {
					current.last = next.last
				}
				continue
			}
		}
		merged = append(merged, next)
	}

	var blocks []netip.Prefix
	for _, run := range merged {
		blocks = appendBlocks(blocks, run.first, run.last)
	}
	return blocks
}

// appendBlocks adds the fewest CIDR blocks that cover first to last: each time the largest one
// that starts at first and ends no later than last.
func appendBlocks(blocks []netip.Prefix, first, last netip.Addr) []netip.Prefix {
	for {
		block := netip.PrefixFrom(first, first.BitLen())
		for bits := range first.BitLen() {
			candidate := netip.PrefixFrom(first, bits)
			if candidate.Masked().Addr() == first && Last(candidate).Compare(last) <= 0 {
				block = candidate
				break
			}
		}
		blocks = append(blocks, block)
		end := Last(block)
		if end == last {
			return blocks
		}
		first = end.Next()
	}
}

// Last is the last address of a block.
func Last(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Masked().Addr().AsSlice()
	for bit := prefix.Bits(); bit < len(bytes)*8; bit++ {
		bytes[bit/8] |= 0x80 >> (bit % 8)
	}
	last, _ := netip.AddrFromSlice(bytes)
	return last
}
