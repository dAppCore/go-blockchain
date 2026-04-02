// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

// DNS record DTOs for go-lns integration.
// These will be replaced by dappco.re/go/core/dns when that package ships.
// For now they allow go-blockchain to produce DNS-ready data from chain aliases.

// DNSRecord holds a resolved .lthn name with all record types.
//
//	record := blockchain.DNSRecord{Name: "charon.lthn", A: []string{"10.69.69.165"}}
type DNSRecord struct {
	Name string   `json:"name"`
	A    []string `json:"a,omitempty"`
	AAAA []string `json:"aaaa,omitempty"`
	TXT  []string `json:"txt,omitempty"`
	NS   []string `json:"ns,omitempty"`
}

// ResolveDNSFromAliases converts chain aliases + HSD records into DNS records.
// This is the bridge between go-blockchain (aliases) and go-lns (DNS serving).
//
//	records := blockchain.ResolveDNSFromAliases(aliases, hsdClient)
func ResolveDNSFromAliases(aliases []chainAlias, hsdFetch func(name string) *DNSRecord) []DNSRecord {
	var records []DNSRecord
	for _, a := range aliases {
		hnsName := a.name
		if a.hnsOverride != "" {
			hnsName = a.hnsOverride
		}
		if rec := hsdFetch(hnsName); rec != nil {
			records = append(records, *rec)
		}
	}
	return records
}

type chainAlias struct {
	name        string
	hnsOverride string
}
