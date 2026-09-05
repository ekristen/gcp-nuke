package resources

import (
	"context"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/cloudkms/v1"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const KMSCryptoKeyResource = "KMSCryptoKey"

func init() {
	registry.Register(&registry.Registration{
		Name:     KMSCryptoKeyResource,
		Scope:    nuke.Project,
		Resource: &KMSCryptoKey{},
		Lister:   &KMSCryptoKeyLister{},
		DependsOn: []string{
			KMSKeyVersionResource,
		},
	})
}

type KMSCryptoKeyLister struct {
	svc *cloudkms.Service
}

// ListCryptoKeys returns every crypto key across every key ring in the region, alongside the key
// ring it belongs to. It is shared with the key version lister.
func (l *KMSCryptoKeyLister) ListCryptoKeys(ctx context.Context, opts *nuke.ListerOpts) (
	[]*cloudkms.CryptoKey, error,
) {
	keyRingLister := &KMSKeyRingLister{}
	keyRings, err := keyRingLister.ListKeyRings(ctx, opts)
	if err != nil {
		return nil, err
	}
	l.svc = keyRingLister.svc

	var cryptoKeys []*cloudkms.CryptoKey
	for _, keyRing := range keyRings {
		err := l.svc.Projects.Locations.KeyRings.CryptoKeys.List(keyRing.Name).Pages(ctx,
			func(page *cloudkms.ListCryptoKeysResponse) error {
				cryptoKeys = append(cryptoKeys, page.CryptoKeys...)
				return nil
			})
		if err != nil {
			// One unreadable key ring should not discard the keys found in the others.
			logrus.WithError(err).WithField("key_ring", keyRing.Name).
				Error("unable to list kms crypto keys")
			continue
		}
	}

	return cryptoKeys, nil
}

func (l *KMSCryptoKeyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudkms.googleapis.com", KMSCryptoKeyResource); err != nil {
		return resources, err
	}

	cryptoKeys, err := l.ListCryptoKeys(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, cryptoKey := range cryptoKeys {
		resources = append(resources, &KMSCryptoKey{
			svc:      l.svc,
			fullName: ptr.String(cryptoKey.Name),
			Name:     ptr.String(kmsShortName(cryptoKey.Name)),
			Keyring:  ptr.String(kmsKeyRingOf(cryptoKey.Name)),
			Purpose:  ptr.String(cryptoKey.Purpose),
			Rotation: ptr.String(cryptoKey.RotationPeriod),
		})
	}

	return resources, nil
}

type KMSCryptoKey struct {
	svc      *cloudkms.Service
	fullName *string
	Name     *string
	Keyring  *string
	Purpose  *string
	Rotation *string
}

// Remove deletes the crypto key. Cloud KMS only permits this once every version of the key has been
// deleted, which is why this resource depends on KMSKeyVersion.
func (r *KMSCryptoKey) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.Delete(*r.fullName).Context(ctx).Do()
	return err
}

func (r *KMSCryptoKey) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *KMSCryptoKey) String() string {
	return *r.Name
}
