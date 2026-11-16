package tools

import (
	"context"
	"strings"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListDomainDNSRecordsInput is the input of list_domain_dns_records.
type ListDomainDNSRecordsInput struct {
	DomainRef
	Type string `json:"type,omitempty" jsonschema:"only records of this type, e.g. A, MX, TXT"`
}

// ListDomainDNSRecordsOutput is the output of list_domain_dns_records.
type ListDomainDNSRecordsOutput struct {
	DomainID    int              `json:"domain_id"`
	DomainName  string           `json:"domain_name"`
	NameServers []api.NameServer `json:"name_servers" jsonschema:"the domain's delegated nameservers; if these are not Dreamscape's, the records below are not live"`
	Records     []api.DNSRecord  `json:"records"`
	Note        string           `json:"note,omitempty"`
}

func addListDomainDNSRecords(s *mcp.Server, d *Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_domain_dns_records",
		Description: "List DNS records hosted on Dreamscape (Crazy Domains) DNS for a domain, by domain_id or domain_name. " +
			"Most of these domains are delegated to DigitalOcean DNS, so this is often empty; the domain's nameservers are " +
			"included so you can tell where its DNS is actually served.",
		Annotations: readOnly("List domain DNS records"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDomainDNSRecordsInput) (*mcp.CallToolResult, ListDomainDNSRecordsOutput, error) {
		id, err := resolveDomainID(ctx, d.Client, in.DomainRef)
		if err != nil {
			return nil, ListDomainDNSRecordsOutput{}, err
		}
		dom, err := d.Client.GetDomain(ctx, id)
		if err != nil {
			return nil, ListDomainDNSRecordsOutput{}, err
		}
		recs, err := d.Client.ListDNSRecords(ctx, id, strings.ToUpper(strings.TrimSpace(in.Type)))
		if err != nil {
			return nil, ListDomainDNSRecordsOutput{}, err
		}
		out := ListDomainDNSRecordsOutput{
			DomainID:    dom.ID,
			DomainName:  dom.DomainName,
			NameServers: orEmpty(dom.NameServers),
			Records:     orEmpty(recs),
		}
		if len(recs) == 0 {
			var hosts []string
			for _, ns := range dom.NameServers {
				hosts = append(hosts, ns.Host)
			}
			if len(hosts) > 0 {
				out.Note = "No DNS records hosted at Dreamscape. The domain is delegated to: " + strings.Join(hosts, ", ")
			} else {
				out.Note = "No DNS records hosted at Dreamscape and no nameservers are set."
			}
		}
		return nil, out, nil
	})
}
