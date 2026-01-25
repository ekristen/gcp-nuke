package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"

	admin "cloud.google.com/go/firestore/apiv1/admin"
	"cloud.google.com/go/firestore/apiv1/admin/adminpb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const FirestoreDatabaseResource = "FirestoreDatabase"

func init() {
	registry.Register(&registry.Registration{
		Name:     FirestoreDatabaseResource,
		Scope:    nuke.Project,
		Resource: &FirestoreDatabase{},
		Lister:   &FirestoreDatabaseLister{},
	})
}

type FirestoreDatabaseLister struct {
	svc *admin.FirestoreAdminClient
}

func (l *FirestoreDatabaseLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *FirestoreDatabaseLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource
	if err := opts.BeforeList(nuke.Global, "firestore.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = admin.NewFirestoreAdminClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &adminpb.ListDatabasesRequest{
		Parent: fmt.Sprintf("projects/%s", *opts.Project),
	}

	resp, err := l.svc.ListDatabases(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, db := range resp.Databases {
		nameParts := strings.Split(db.Name, "/")

		resources = append(resources, &FirestoreDatabase{
			svc:      l.svc,
			project:  opts.Project,
			fullName: ptr.String(db.Name),
			Name:     ptr.String(nameParts[len(nameParts)-1]),
			Location: ptr.String(db.LocationId),
		})
	}

	return resources, nil
}

type FirestoreDatabase struct {
	svc      *admin.FirestoreAdminClient
	project  *string
	fullName *string
	Name     *string
	Location *string
}

func (r *FirestoreDatabase) Remove(ctx context.Context) error {
	_, err := r.svc.DeleteDatabase(ctx, &adminpb.DeleteDatabaseRequest{
		Name: *r.fullName,
	})
	return err
}

func (r *FirestoreDatabase) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FirestoreDatabase) String() string {
	return *r.Name
}
