// Command hostingranges writes internal/biz/hostingranges.txt: the address ranges of cloud and
// hosting providers, which auto_check never lets through without a puzzle. People rarely browse
// from a data centre; bots usually run there. Refresh the list about once a month, from the
// service root, and commit the file:
//
//	go run ./cmd/hostingranges
//
// A source that cannot be fetched or read is skipped with a warning, and the file lists the others.
// CDN and proxy ranges (CloudFront, Cloudflare, Google's own services, iCloud Private Relay) are left
// out on purpose: real people come through them.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"challenge/internal/pkg/iprange"
)

// maxBody bounds what one source may send; the largest, AWS's, is a few MB.
const maxBody = 32 << 20

type source struct {
	name, url string
	// parse returns the blocks a source lists for its cloud.
	parse func(body []byte) ([]string, error)
}

var sources = []source{
	{"AWS EC2", "https://ip-ranges.amazonaws.com/ip-ranges.json", parseAWS},
	{"Google Cloud", "https://www.gstatic.com/ipranges/cloud.json", parseGoogleCloud},
	{"Oracle Cloud", "https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json", parseOracle},
	{"DigitalOcean", "https://digitalocean.com/geo/google.csv", parseFirstColumn},
	{"Linode", "https://geoip.linode.com/", parseFirstColumn},
}

func main() {
	out := flag.String("out", "internal/biz/hostingranges.txt", "file to write")
	flag.Parse()

	client := &http.Client{Timeout: time.Minute}
	var all []netip.Prefix
	var header []string
	for _, src := range sources {
		blocks, err := fetch(client, src)
		if err != nil {
			log.Printf("warning: %s skipped: %v", src.name, err)
			header = append(header, fmt.Sprintf("# %s, %s: skipped (%v)", src.name, src.url, err))
			continue
		}
		log.Printf("%s: %d ranges", src.name, len(blocks))
		header = append(header, fmt.Sprintf("# %s, %s: %d ranges", src.name, src.url, len(blocks)))
		all = append(all, blocks...)
	}
	if len(all) == 0 {
		log.Fatal("no source could be read; the file is left as it was")
	}
	merged := iprange.Merge(all)

	var file bytes.Buffer
	file.WriteString("# Address ranges of cloud and hosting providers, merged and sorted: auto_check never lets an\n")
	file.WriteString("# address in them through without a puzzle (see hosting.go). Written by\n")
	fmt.Fprintf(&file, "# `go run ./cmd/hostingranges` on %s (UTC); refresh it about once a month.\n", time.Now().UTC().Format(time.DateOnly))
	for _, line := range header {
		file.WriteString(line + "\n")
	}
	fmt.Fprintf(&file, "# %d ranges after merging.\n", len(merged))
	for _, block := range merged {
		file.WriteString(block.String() + "\n")
	}
	if err := os.WriteFile(*out, file.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: %d ranges", *out, len(merged))
}

// fetch downloads a source and returns its blocks, each a valid CIDR block.
func fetch(client *http.Client, src source) ([]netip.Prefix, error) {
	response, err := client.Get(src.url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, errors.New("larger than 32 MB")
	}
	texts, err := src.parse(body)
	if err != nil {
		return nil, err
	}
	blocks := make([]netip.Prefix, 0, len(texts))
	invalid := 0
	for _, text := range texts {
		block, err := netip.ParsePrefix(strings.TrimSpace(text))
		if err != nil {
			invalid++
			continue
		}
		blocks = append(blocks, block.Masked())
	}
	if invalid > 0 {
		log.Printf("warning: %s: %d entries are not CIDR blocks, left out", src.name, invalid)
	}
	if len(blocks) == 0 {
		return nil, errors.New("no ranges in it")
	}
	return blocks, nil
}

// parseAWS reads ip-ranges.json, keeping service EC2 only: the other services are AWS's own
// endpoints and CloudFront, where people's requests come from too.
func parseAWS(body []byte) ([]string, error) {
	var ranges struct {
		Prefixes []struct {
			Prefix  string `json:"ip_prefix"`
			Service string `json:"service"`
		} `json:"prefixes"`
		IPv6Prefixes []struct {
			Prefix  string `json:"ipv6_prefix"`
			Service string `json:"service"`
		} `json:"ipv6_prefixes"`
	}
	if err := json.Unmarshal(body, &ranges); err != nil {
		return nil, err
	}
	var blocks []string
	for _, entry := range ranges.Prefixes {
		if entry.Service == "EC2" {
			blocks = append(blocks, entry.Prefix)
		}
	}
	for _, entry := range ranges.IPv6Prefixes {
		if entry.Service == "EC2" {
			blocks = append(blocks, entry.Prefix)
		}
	}
	return blocks, nil
}

// parseGoogleCloud reads cloud.json: the ranges of Google Cloud customers, not Google's own services.
func parseGoogleCloud(body []byte) ([]string, error) {
	var ranges struct {
		Prefixes []struct {
			IPv4 string `json:"ipv4Prefix"`
			IPv6 string `json:"ipv6Prefix"`
		} `json:"prefixes"`
	}
	if err := json.Unmarshal(body, &ranges); err != nil {
		return nil, err
	}
	var blocks []string
	for _, entry := range ranges.Prefixes {
		for _, block := range []string{entry.IPv4, entry.IPv6} {
			if block != "" {
				blocks = append(blocks, block)
			}
		}
	}
	return blocks, nil
}

// parseOracle reads public_ip_ranges.json, keeping the blocks tagged OCI, where customers' machines
// get their addresses; OSN and object storage are Oracle's own endpoints.
func parseOracle(body []byte) ([]string, error) {
	var ranges struct {
		Regions []struct {
			CIDRs []struct {
				CIDR string   `json:"cidr"`
				Tags []string `json:"tags"`
			} `json:"cidrs"`
		} `json:"regions"`
	}
	if err := json.Unmarshal(body, &ranges); err != nil {
		return nil, err
	}
	var blocks []string
	for _, region := range ranges.Regions {
		for _, entry := range region.CIDRs {
			if slices.Contains(entry.Tags, "OCI") {
				blocks = append(blocks, entry.CIDR)
			}
		}
	}
	return blocks, nil
}

// parseFirstColumn reads a geofeed (RFC 8805): CSV whose first column is the block, with # comments.
func parseFirstColumn(body []byte) ([]string, error) {
	var blocks []string
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		block, _, _ := strings.Cut(line, ",")
		blocks = append(blocks, block)
	}
	return blocks, scanner.Err()
}
