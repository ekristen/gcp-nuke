package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"

	"google.golang.org/api/cloudkms/v1"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const KMSKeyRingResource = "KMSKeyRing"

func init() {
	registry.Register(&registry.Registration{
		Name:     KMSKeyRingResource,
		Scope:    nuke.Project,
		Resource: &KMSKeyRing{},
		Lister:   &KMSKeyRingLister{},
		DependsOn: []string{
			KMSCryptoKeyResource,
		},
	})
}

// kmsShortName returns the last segment of a fully qualified Cloud KMS resource name.
func kmsShortName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// kmsKeyRingOf returns the key ring segment of a fully qualified crypto key or key version name,
// which look like projects/P/locations/L/keyRings/R/cryptoKeys/K[/cryptoKeyVersions/V].
func kmsKeyRingOf(name string) string {
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if part == "keyRings" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

type KMSKeyRingLister struct {
	svc *cloudkms.Service
}

func (l *KMSKeyRingLister) newService(ctx context.Context, opts *nuke.ListerOpts) error {
	if l.svc != nil {
		return nil
	}

	var err error
	l.svc, err = cloudkms.NewService(ctx, opts.ClientOptions...)
	return err
}

// ListKeyRings returns the key rings in the region being nuked. It is shared with the crypto key
// and key version listers, which both need to walk the key rings first.
func (l *KMSKeyRingLister) ListKeyRings(ctx context.Context, opts *nuke.ListerOpts) ([]*cloudkms.KeyRing, error) {
	if err := l.newService(ctx, opts); err != nil {
		return nil, err
	}

	var keyRings []*cloudkms.KeyRing
	parent := fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region)

	err := l.svc.Projects.Locations.KeyRings.List(parent).Pages(ctx,
		func(page *cloudkms.ListKeyRingsResponse) error {
			keyRings = append(keyRings, page.KeyRings...)
			return nil
		})
	if err != nil {
		return nil, err
	}

	return keyRings, nil
}

func (l *KMSKeyRingLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudkms.googleapis.com", KMSKeyRingResource); err != nil {
		return resources, err
	}

	keyRings, err := l.ListKeyRings(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, keyRing := range keyRings {
		resources = append(resources, &KMSKeyRing{
			svc:       l.svc,
			fullName:  ptr.String(keyRing.Name),
			Name:      ptr.String(kmsShortName(keyRing.Name)),
			CreatedAt: ptr.String(keyRing.CreateTime),
		})
	}

	return resources, nil
}

type KMSKeyRing struct {
	svc       *cloudkms.Service
	fullName  *string
	Name      *string
	CreatedAt *string
}

// Remove deletes the key ring. Cloud KMS only permits this once every crypto key and import job in
// the ring has been deleted, which is why this resource depends on KMSCryptoKey.
func (r *KMSKeyRing) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.KeyRings.Delete(*r.fullName).Context(ctx).Do()
	return err
}

func (r *KMSKeyRing) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *KMSKeyRing) String() string {
	return *r.Name
}
