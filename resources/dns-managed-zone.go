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

const DNSManagedZoneResource = "DNSManagedZone"

func init() {
	registry.Register(&registry.Registration{
		Name:     DNSManagedZoneResource,
		Scope:    nuke.Project,
		Resource: &DNSManagedZone{},
		Lister:   &DNSManagedZoneLister{},
	})
}

type DNSManagedZoneLister struct {
	svc *dns.Service
}

func (l *DNSManagedZoneLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource

	if err := opts.BeforeList(nuke.Global, "dns.googleapis.com", DNSManagedZoneResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dns.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := l.svc.ManagedZones.List(*opts.Project)
	if err := req.Pages(ctx, func(page *dns.ManagedZonesListResponse) error {
		for _, zone := range page.ManagedZones {
			resources = append(resources, &DNSManagedZone{
				svc:          l.svc,
				project:      opts.Project,
				Name:         ptr.String(zone.Name),
				DNSName:      ptr.String(zone.DnsName),
				Visibility:   ptr.String(zone.Visibility),
				CreationTime: ptr.String(zone.CreationTime),
			})
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type DNSManagedZone struct {
	svc          *dns.Service
	project      *string
	Name         *string `description:"Name of the managed zone"`
	DNSName      *string `description:"DNS name of the managed zone"`
	CreationTime *string `description:"Creation time of the managed zone"`
	Visibility   *string
	Labels       map[string]string `property:"tagPrefix=label" description:"Labels of the managed zone"`
}

func (r *DNSManagedZone) Remove(ctx context.Context) error {
	if err := r.removeRecords(ctx); err != nil {
		return err
	}

	return r.svc.ManagedZones.Delete(*r.project, *r.Name).Do()
}

func (r *DNSManagedZone) removeRecords(ctx context.Context) error {
	var deletions []*dns.ResourceRecordSet

	req := r.svc.ResourceRecordSets.List(*r.project, *r.Name)
	if err := req.Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, rrset := range page.Rrsets {
			if rrset.Name == *r.DNSName && (rrset.Type == "SOA" || rrset.Type == "NS") {
				continue
			}
			deletions = append(deletions, rrset)
		}
		return nil
	}); err != nil {
		return err
	}

	if len(deletions) == 0 {
		return nil
	}

	change := &dns.Change{
		Deletions: deletions,
	}

	_, err := r.svc.Changes.Create(*r.project, *r.Name, change).Context(ctx).Do()
	return err
}

func (r *DNSManagedZone) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DNSManagedZone) String() string {
	return *r.Name
}
