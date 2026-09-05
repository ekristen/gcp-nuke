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
		err := l.svc.Projects.Locations.KeyRings.CryptoKeys.CryptoKeyVersions.List(cryptoKey.Name).
			Pages(ctx, func(page *cloudkms.ListCryptoKeyVersionsResponse) error {
				for _, keyVersion := range page.CryptoKeyVersions {
					resources = append(resources, &KMSKeyVersion{
						svc:      l.svc,
						fullName: ptr.String(keyVersion.Name),
						imported: keyVersion.ImportTime != "",
						Name:     ptr.String(kmsShortName(cryptoKey.Name)),
						Keyring:  ptr.String(kmsKeyRingOf(keyVersion.Name)),
						Version:  ptr.String(kmsShortName(keyVersion.Name)),
						State:    ptr.String(keyVersion.State),
					})
				}
				return nil
			})
		if err != nil {
			// One unreadable key should not discard the versions found on the others.
			logrus.WithError(err).WithField("key", cryptoKey.Name).
				Error("unable to list kms key versions")
			continue
		}
	}

	return resources, nil
}

type KMSKeyVersion struct {
	svc      *cloudkms.Service
	fullName *string
	imported bool
	Name     *string
	Keyring  *string
	Version  *string
	State    *string
}

// deletable reports whether the version is in a state Cloud KMS allows to be deleted outright.
func (r *KMSKeyVersion) deletable() bool {
	switch *r.State {
	case kmsStateDestroyed, kmsStateImportFailed, kmsStateGenerationFailed:
		return true
	}
	return false
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

	_, err := r.svc.Projects.Locations.KeyRings.CryptoKeys.CryptoKeyVersions.
		Destroy(*r.fullName, &cloudkms.DestroyCryptoKeyVersionRequest{}).Context(ctx).Do()
	return err
}

func (r *KMSKeyVersion) Filter() error {
	switch *r.State {
	case kmsStateDestroyScheduled:
		return fmt.Errorf("key version is already scheduled for destruction")
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
