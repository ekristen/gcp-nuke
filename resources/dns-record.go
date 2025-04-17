package resources

import (
	"context"

	"github.com/gotidy/ptr"

	"google.golang.org/api/dns/v1"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const DNSRecordResource = "DNSRecord"

func init() {
	registry.Register(&registry.Registration{
		Name:     DNSRecordResource,
		Scope:    nuke.Project,
		Resource: &DNSRecord{},
		Lister:   &DNSRecordLister{},
	})
}

type DNSRecordLister struct {
	svc *dns.Service
}

func (l *DNSRecordLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource

	if err := opts.BeforeList(nuke.Global, "dns.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dns.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	// First list all managed zones
	zonesReq := l.svc.ManagedZones.List(*opts.Project)
	if err := zonesReq.Pages(ctx, func(page *dns.ManagedZonesListResponse) error {
		for _, zone := range page.ManagedZones {
			// For each zone, list all record sets
			recordsReq := l.svc.ResourceRecordSets.List(*opts.Project, zone.Name)
			if err := recordsReq.Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
				for _, record := range page.Rrsets {
					resources = append(resources, &DNSRecord{
						svc:          l.svc,
						project:      opts.Project,
						zoneName:     ptr.String(zone.Name),
						Name:         ptr.String(record.Name),
						Type:         ptr.String(record.Type),
						TTL:          ptr.Int64(record.Ttl),
						Rrdatas:      record.Rrdatas,
						CreationTime: ptr.String(zone.CreationTime),
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

type DNSRecord struct {
	svc          *dns.Service
	project      *string
	zoneName     *string
	Name         *string `description:"Name of the DNS record"`
	Type         *string `description:"Type of the DNS record"`
	TTL          *int64  `description:"Time to live of the DNS record"`
	Rrdatas      []string
	CreationTime *string `description:"Creation time of the DNS record"`
}

func (r *DNSRecord) Remove(ctx context.Context) error {
	// Create a change to delete the record
	change := &dns.Change{
		Deletions: []*dns.ResourceRecordSet{
			{
				Name:    *r.Name,
				Type:    *r.Type,
				Ttl:     *r.TTL,
				Rrdatas: r.Rrdatas,
			},
		},
	}

	// Apply the change to delete the record
	_, err := r.svc.Changes.Create(*r.project, *r.zoneName, change).Do()
	return err
}

// TODO: implement a HandlwWait method to poll until the resource has been deleted

func (r *DNSRecord) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DNSRecord) String() string {
	return *r.Name
}
