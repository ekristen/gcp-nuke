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

const KMSKeyVersionResource = "KMSKeyVersion"

// Cloud KMS key version states. The generated REST client models state as a plain string, so these
// are declared here rather than imported from a protobuf enum.
const (
	kmsStateEnabled                    = "ENABLED"
	kmsStateDisabled                   = "DISABLED"
	kmsStateDestroyed                  = "DESTROYED"
	kmsStateDestroyScheduled           = "DESTROY_SCHEDULED"
	kmsStateImportFailed               = "IMPORT_FAILED"
	kmsStateGenerationFailed           = "GENERATION_FAILED"
	kmsStatePendingGeneration          = "PENDING_GENERATION"
	kmsStatePendingImport              = "PENDING_IMPORT"
	kmsStatePendingExternalDestruction = "PENDING_EXTERNAL_DESTRUCTION"
)

func init() {
	registry.Register(&registry.Registration{
		Name:              KMSKeyVersionResource,
		Scope:             nuke.Project,
		Resource:          &KMSKeyVersion{},
		Lister:            &KMSKeyVersionLister{},
		DeprecatedAliases: []string{"KMSKey"},
		DependsOn: []string{
			AlloyDBClusterResource,
			ArtifactRegistryRepositoryResource,
			BigQueryDatasetResource,
			CloudSQLInstanceResource,
			ComposerEnvironmentResource,
			ComputeDiskResource,
			DataprocClusterResource,
			FilestoreInstanceResource,
			GKEClusterResource,
			SecretManagerSecretResource,
			SpannerDatabaseResource,
			StorageBucketResource,
		},
	})
}

type KMSKeyVersionLister struct {
	svc *cloudkms.Service
}

func (l *KMSKeyVersionLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudkms.googleapis.com", KMSKeyVersionResource); err != nil {
		return resources, err
	}

	cryptoKeyLister := &KMSCryptoKeyLister{}
	cryptoKeys, err := cryptoKeyLister.ListCryptoKeys(ctx, opts)
	if err != nil {
		return nil, err
	}
	l.svc = cryptoKeyLister.svc

	for _, cryptoKey := range cryptoKeys {
		for _, keyVersion := range cryptoKey.versions {
			resources = append(resources, &KMSKeyVersion{
				svc:         l.svc,
				fullName:    ptr.String(keyVersion.Name),
				keyName:     cryptoKey.key.Name,
				keyRotation: cryptoKey.key.RotationPeriod,
				imported:    keyVersion.ImportTime != "",
				Name:        ptr.String(kmsShortName(cryptoKey.key.Name)),
				Keyring:     ptr.String(kmsKeyRingOf(keyVersion.Name)),
				Version:     ptr.String(kmsShortName(keyVersion.Name)),
				State:       ptr.String(keyVersion.State),
				DestroyTime: ptr.String(keyVersion.DestroyTime),
			})
		}
	}

	return resources, nil
}

type KMSKeyVersion struct {
	svc         *cloudkms.Service
	fullName    *string
	keyName     string
	keyRotation string
	imported    bool
	Name        *string
	Keyring     *string
	Version     *string
	State       *string
	DestroyTime *string
}

// kmsVersionDeletable reports whether a version state is one Cloud KMS allows to be deleted
// outright. Any other state has to be destroyed first, which cannot complete in the same run.
func kmsVersionDeletable(state string) bool {
	switch state {
	case kmsStateDestroyed, kmsStateImportFailed, kmsStateGenerationFailed:
		return true
	}
	return false
}

func (r *KMSKeyVersion) deletable() bool {
	return kmsVersionDeletable(*r.State)
}

// Remove advances the version towards removal. A live version must first be destroyed, which
// schedules its key material for destruction; only once it reaches DESTROYED can it be deleted.
// Those are separate runs, because the destruction period defaults to 24 hours.
func (r *KMSKeyVersion) Remove(ctx context.Context) error {
	if r.deletable() {
		_, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.CryptoKeyVersions.
			Delete(*r.fullName).Context(ctx).Do()
		return err
	}

	// Destroying only schedules the version, and the wait is at least 24 hours. Automatic rotation
	// can create a replacement version during that window -- and the minimum rotation period is
	// also 24 hours -- which would restart the wait and keep the key from ever being deleted. So
	// the schedule is cleared on the parent key before the destroy is requested.
	if err := r.clearKeyRotation(ctx); err != nil {
		return err
	}

	_, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.CryptoKeyVersions.
		Destroy(*r.fullName, &cloudkms.DestroyCryptoKeyVersionRequest{}).Context(ctx).Do()
	return err
}

// clearKeyRotation removes the automatic rotation schedule from the version's parent key. The API
// clears fields that are named in the update mask but left unset in the body.
func (r *KMSKeyVersion) clearKeyRotation(ctx context.Context) error {
	if r.keyRotation == "" {
		return nil
	}

	if _, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.
		Patch(r.keyName, &cloudkms.CryptoKey{}).
		UpdateMask("rotationPeriod,nextRotationTime").
		Context(ctx).Do(); err != nil {
		return fmt.Errorf("unable to clear rotation schedule on %s: %w", r.keyName, err)
	}

	logrus.WithField("key", r.keyName).Debug("cleared rotation schedule before destroying version")
	return nil
}

func (r *KMSKeyVersion) Filter() error {
	switch *r.State {
	case kmsStateDestroyScheduled:
		// The destruction period is fixed when the key is created and cannot be shortened, so
		// nothing can be done until it elapses.
		return fmt.Errorf("key version is scheduled for destruction on %s", *r.DestroyTime)
	case kmsStatePendingGeneration, kmsStatePendingImport:
		return fmt.Errorf("key version is still being generated or imported")
	case kmsStatePendingExternalDestruction:
		return fmt.Errorf("key version is already pending external destruction")
	}

	// Cloud KMS refuses to delete a version that was successfully imported, so once such a version
	// has been destroyed there is nothing further to do with it.
	if r.imported && r.deletable() {
		return fmt.Errorf("imported key versions cannot be deleted")
	}

	return nil
}

func (r *KMSKeyVersion) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *KMSKeyVersion) String() string {
	return fmt.Sprintf("%s/%s/%s", *r.Keyring, *r.Name, *r.Version)
}
