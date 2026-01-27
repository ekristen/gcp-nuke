package resources

import (
	"context"
	"fmt"

	"github.com/gotidy/ptr"

	"google.golang.org/api/dns/v1"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const DNSRecordSetResource = "DNSRecordSet"

func init() {
	registry.Register(&registry.Registration{
		Name:     DNSRecordSetResource,
		Scope:    nuke.Project,
		Resource: &DNSRecordSet{},
		Lister:   &DNSRecordSetLister{},
	})
}

type DNSRecordSetLister struct {
	svc *dns.Service
}

func (l *DNSRecordSetLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource

	if err := opts.BeforeList(nuke.Global, "dns.googleapis.com", DNSRecordSetResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dns.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	zonesReq := l.svc.ManagedZones.List(*opts.Project)
	if err := zonesReq.Pages(ctx, func(zonesPage *dns.ManagedZonesListResponse) error {
		for _, zone := range zonesPage.ManagedZones {
			rrsetReq := l.svc.ResourceRecordSets.List(*opts.Project, zone.Name)
			if err := rrsetReq.Pages(ctx, func(rrsetPage *dns.ResourceRecordSetsListResponse) error {
				for _, rrset := range rrsetPage.Rrsets {
					resources = append(resources, &DNSRecordSet{
						svc:     l.svc,
						project: opts.Project,
						Zone:    ptr.String(zone.Name),
						ZoneDNS: ptr.String(zone.DnsName),
						Name:    ptr.String(rrset.Name),
						Type:    ptr.String(rrset.Type),
						TTL:     ptr.Int64(rrset.Ttl),
						RRDatas: rrset.Rrdatas,
					})
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type DNSRecordSet struct {
	svc     *dns.Service
	project *string
	Zone    *string
	ZoneDNS *string
	Name    *string
	Type    *string
	TTL     *int64
	RRDatas []string
}

func (r *DNSRecordSet) Filter() error {
	if *r.Name == *r.ZoneDNS && (*r.Type == "SOA" || *r.Type == "NS") {
		return fmt.Errorf("cannot delete %s record at zone apex", *r.Type)
	}
	return nil
}

func (r *DNSRecordSet) Remove(ctx context.Context) error {
	change := &dns.Change{
		Deletions: []*dns.ResourceRecordSet{
			{
				Name:    *r.Name,
				Type:    *r.Type,
				Ttl:     *r.TTL,
				Rrdatas: r.RRDatas,
			},
		},
	}

	_, err := r.svc.Changes.Create(*r.project, *r.Zone, change).Context(ctx).Do()
	return err
}

func (r *DNSRecordSet) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DNSRecordSet) String() string {
	return fmt.Sprintf("%s -> %s %s", *r.Zone, *r.Name, *r.Type)
}
