package main

import (
	"bytes"
	"fmt"
	"github.com/gertd/go-pluralize"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"strings"
	"text/template"
)

const resourceTemplate = `package resources

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	{{.Service}} "cloud.google.com/go/{{.Service}}/apiv1"
	"cloud.google.com/go/{{.Service}}/apiv1/{{.Service}}pb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const {{.Combined}}Resource = "{{.Combined}}"

func init() {
	registry.Register(&registry.Registration{
		Name:   {{.Combined}}Resource,
		Scope:  nuke.project,
		Lister: &{{.Combined}}Lister{},
	})
}

type {{.Combined}}Lister struct{
	svc *{{.Service}}.{{.ResourceTypeTitlePlural}}Client
}

func (l *{{.Combined}}Lister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = {{.Service}}.New{{.ResourceTypeTitlePlural}}RESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// NOTE: you might have to modify the code below to actually work, this currently does not 
	// inspect the google go sdk instead is a jumping off point
	req := &{{.Service}}pb.List{{.ResourceTypeTitlePlural}}Request{
		project: *opts.project,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &{{.Combined}}{
			svc:     l.svc,
			Name:    resp.Name,
			project: opts.project,
		})
	}

	return resources, nil
}

type {{.Combined}} struct {
	svc  *{{.Service}}.{{.ResourceTypeTitlePlural}}Client
	project *string
	region *string
	Name *string
}

func (r *{{.Combined}}) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &{{.Service}}pb.Delete{{.ResourceTypeTitle}}Request{
		project: *r.Project,		
		Name: *r.Name,
	})
	return err
}

func (r *{{.Combined}}) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *{{.Combined}}) String() string {
	return *r.Name
}
`

func main() {
	args := os.Args[1:]

	if len(args) != 2 {
		fmt.Println("usage: create-resource <service> <resource>")
		os.Exit(1)
	}

	service := args[0]
	resourceType := args[1]

	caser := cases.Title(language.English)
	pluralize := pluralize.NewClient()

	data := struct {
		Service                 string
		ServiceTitle            string
		ResourceType            string
		ResourceTypeTitle       string
		ResourceTypeTitlePlural string
		Combined                string
	}{
		Service:                 strings.ToLower(service),
		ServiceTitle:            caser.String(service),
		ResourceType:            resourceType,
		ResourceTypeTitle:       caser.String(resourceType),
		ResourceTypeTitlePlural: caser.String(pluralize.Plural(resourceType)),
		Combined:                fmt.Sprintf("%s%s", caser.String(service), caser.String(resourceType)),
	}

	tmpl, err := template.New("resource").Parse(resourceTemplate)
	if err != nil {
		panic(err)
	}

	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, data); err != nil {
		panic(err)
	}

	fmt.Println(tpl.String())
}
