package resources

import (
	"context"
	"fmt"

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

// kmsCryptoKey pairs a crypto key with its remaining versions. Cloud KMS refuses to delete a key
// until every version has been deleted, so the version list decides whether the key is removable.
type kmsCryptoKey struct {
	key      *cloudkms.CryptoKey
	versions []*cloudkms.CryptoKeyVersion
}

// blockedBy reports why the key cannot be deleted, or "" if it can be. Versions that are already
// deletable do not block: KMSKeyVersion removes them earlier in the same run via DependsOn, and
// libnuke re-attempts this key afterwards. Only versions that still have to go through the
// destruction period -- a minimum of 24 hours, fixed at key creation -- genuinely block, and they
// cannot clear before the next run.
func (k *kmsCryptoKey) blockedBy() string {
	pending, imported := 0, 0
	for _, version := range k.versions {
		switch {
		case version.ImportTime != "" && kmsVersionDeletable(version.State):
			imported++
		case kmsVersionDeletable(version.State):
			// removed earlier in this run
		default:
			pending++
		}
	}

	if imported > 0 {
		return fmt.Sprintf("%d imported version(s) that cannot be deleted", imported)
	}
	if pending > 0 {
		return fmt.Sprintf("%d version(s) awaiting destruction", pending)
	}
	return ""
}

type KMSCryptoKeyLister struct {
	svc *cloudkms.Service
}

// ListCryptoKeys returns every crypto key across every key ring in the region, together with the
// versions still present on each. It is shared with the key version lister.
func (l *KMSCryptoKeyLister) ListCryptoKeys(ctx context.Context, opts *nuke.ListerOpts) (
	[]*kmsCryptoKey, error,
) {
	keyRingLister := &KMSKeyRingLister{}
	keyRings, err := keyRingLister.ListKeyRings(ctx, opts)
	if err != nil {
		return nil, err
	}
	l.svc = keyRingLister.svc

	var cryptoKeys []*kmsCryptoKey
	for _, keyRing := range keyRings {
		var keys []*cloudkms.CryptoKey
		err := l.svc.Projects.Locations.KeyRings.CryptoKeys.List(keyRing.Name).Pages(ctx,
			func(page *cloudkms.ListCryptoKeysResponse) error {
				keys = append(keys, page.CryptoKeys...)
				return nil
			})
		if err != nil {
			// One unreadable key ring should not discard the keys found in the others.
			logrus.WithError(err).WithField("key_ring", keyRing.Name).
				Error("unable to list kms crypto keys")
			continue
		}

		for _, key := range keys {
			var versions []*cloudkms.CryptoKeyVersion
			err := l.svc.Projects.Locations.KeyRings.CryptoKeys.CryptoKeyVersions.List(key.Name).
				Pages(ctx, func(page *cloudkms.ListCryptoKeyVersionsResponse) error {
					versions = append(versions, page.CryptoKeyVersions...)
					return nil
				})
			if err != nil {
				// One unreadable key should not discard the others.
				logrus.WithError(err).WithField("key", key.Name).
					Error("unable to list kms key versions")
				continue
			}

			cryptoKeys = append(cryptoKeys, &kmsCryptoKey{key: key, versions: versions})
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
			fullName: ptr.String(cryptoKey.key.Name),
			blocked:  cryptoKey.blockedBy(),
			Name:     ptr.String(kmsShortName(cryptoKey.key.Name)),
			Keyring:  ptr.String(kmsKeyRingOf(cryptoKey.key.Name)),
			Purpose:  ptr.String(cryptoKey.key.Purpose),
			Rotation: ptr.String(cryptoKey.key.RotationPeriod),
		})
	}

	return resources, nil
}

type KMSCryptoKey struct {
	svc             *cloudkms.Service
	fullName        *string
	blocked         string
	rotationCleared bool
	Name            *string
	Keyring         *string
	Purpose         *string
	Rotation        *string
}

func (r *KMSCryptoKey) rotates() bool {
	return r.Rotation != nil && *r.Rotation != ""
}

func (r *KMSCryptoKey) Filter() error {
	// A key that still rotates always has work to do, even when the delete itself cannot succeed
	// yet: the schedule has to be cleared so rotation cannot mint a replacement version while the
	// existing ones sit out their destruction period. Filtering it here would leave the schedule
	// live for the whole window, which is how a key with a short rotation period never converges.
	if r.blocked != "" && !r.rotates() {
		return fmt.Errorf("key has %s", r.blocked)
	}
	return nil
}

// Remove clears the rotation schedule and deletes the crypto key. Cloud KMS only permits the delete
// once every version has been deleted, which is why this resource depends on KMSKeyVersion.
func (r *KMSCryptoKey) Remove(ctx context.Context) error {
	// A key with automatic rotation scheduled cannot be deleted, so the schedule is cleared first.
	// Fields named in the update mask but left unset in the body are cleared by the API.
	if r.rotates() && !r.rotationCleared {
		if _, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.
			Patch(*r.fullName, &cloudkms.CryptoKey{}).
			UpdateMask("rotationPeriod,nextRotationTime").
			Context(ctx).Do(); err != nil {
			return fmt.Errorf("unable to clear rotation schedule: %w", err)
		}
		r.rotationCleared = true
		logrus.WithField("key", *r.fullName).Debug("cleared rotation schedule")
	}

	// Reached only for a key kept unfiltered by its rotation schedule. The schedule is now gone, so
	// the next run has nothing left to do here but wait out the destruction period.
	if r.blocked != "" {
		return fmt.Errorf("rotation schedule cleared, but key still has %s", r.blocked)
	}

	_, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.Delete(*r.fullName).Context(ctx).Do()
	return err
}

func (r *KMSCryptoKey) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *KMSCryptoKey) String() string {
	return *r.Name
}
