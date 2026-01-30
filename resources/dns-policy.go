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

const DNSPolicyResource = "DNSPolicy"

func init() {
	registry.Register(&registry.Registration{
		Name:     DNSPolicyResource,
		Scope:    nuke.Project,
		Resource: &DNSPolicy{},
		Lister:   &DNSPolicyLister{},
	})
}

type DNSPolicyLister struct {
	svc *dns.Service
}

func (l *DNSPolicyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource

	if err := opts.BeforeList(nuke.Global, "dns.googleapis.com", DNSPolicyResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dns.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := l.svc.Policies.List(*opts.Project)
	if err := req.Pages(ctx, func(page *dns.PoliciesListResponse) error {
		for _, policy := range page.Policies {
			resources = append(resources, &DNSPolicy{
				svc:                     l.svc,
				project:                 opts.Project,
				Name:                    ptr.String(policy.Name),
				Description:             ptr.String(policy.Description),
				EnableInboundForwarding: ptr.Bool(policy.EnableInboundForwarding),
				EnableLogging:           ptr.Bool(policy.EnableLogging),
			})
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type DNSPolicy struct {
	svc                     *dns.Service
	project                 *string
	Name                    *string `description:"Name of the DNS policy"`
	Description             *string `description:"Description of the DNS policy"`
	EnableInboundForwarding *bool   `description:"Whether inbound forwarding is enabled"`
	EnableLogging           *bool   `description:"Whether logging is enabled"`
}

func (r *DNSPolicy) Remove(ctx context.Context) error {
	// First detach the policy from all networks to avoid deletion failure
	// if networks are deleted before the policy
	_, _ = r.svc.Policies.Patch(*r.project, *r.Name, &dns.Policy{
		Networks: []*dns.PolicyNetwork{},
	}).Do()

	return r.svc.Policies.Delete(*r.project, *r.Name).Do()
}

func (r *DNSPolicy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DNSPolicy) String() string {
	return *r.Name
}
