package oras

import (
	"context"
	"net/http"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
	"gopkg.in/yaml.v2"
)

const (
	namespaceName          = "oras"
	extensionName          = "artifacts"
	referrersComponentName = "referrers"
	namespaceUrl           = "https://github.com/oras-project/artifacts-spec/blob/main/manifest-referrers-api.md"
	namespaceDescription   = "ORAS referrers listing API."
)

type orasNamespace struct {
	storageDriver    driver.StorageDriver
	referrersEnabled bool
	gcHandler        orasGCHandler
}

type OrasOptions struct {
	ArtifactsExtComponents []string `yaml:"artifacts,omitempty"`
}

// newOrasNamespace creates a new extension namespace with the name "oras"
func newOrasNamespace(ctx context.Context, storageDriver driver.StorageDriver, options configuration.ExtensionConfig) (extension.Extension, error) {
	optionsYaml, err := yaml.Marshal(options)
	if err != nil {
		return nil, err
	}

	var orasOptions OrasOptions
	err = yaml.Unmarshal(optionsYaml, &orasOptions)
	if err != nil {
		return nil, err
	}

	referrersEnabled := false
	for _, component := range orasOptions.ArtifactsExtComponents {
		if component == referrersComponentName {
			referrersEnabled = true
			break
		}
	}

	orasGCHandler := orasGCHandler{
		artifactManifestIndex: make(map[digest.Digest][]digest.Digest),
		artifactMarkSet:       make(map[digest.Digest]int),
	}

	return &orasNamespace{
		referrersEnabled: referrersEnabled,
		storageDriver:    storageDriver,
		gcHandler:        orasGCHandler,
	}, nil
}

func init() {
	extension.RegisterExtension(namespaceName, newOrasNamespace)
}

// GetManifestHandlers returns a list of manifest handlers that will be registered in the manifest store.
func (o *orasNamespace) GetManifestHandlers(repo distribution.Repository, blobStore distribution.BlobStore) []storage.ManifestHandler {
	if o.referrersEnabled {
		return []storage.ManifestHandler{
			&artifactManifestHandler{
				repository:    repo,
				blobStore:     blobStore,
				storageDriver: o.storageDriver,
			}}
	}

	return []storage.ManifestHandler{}
}

func (o *orasNamespace) GetGarbageCollectionHandlers() []storage.GCExtensionHandler {
	if o.referrersEnabled {
		return []storage.GCExtensionHandler{
			&o.gcHandler,
		}
	}

	return []storage.GCExtensionHandler{}
}

// GetRepositoryRoutes returns a list of extension routes scoped at a repository level
func (d *orasNamespace) GetRepositoryRoutes() []extension.ExtensionRoute {
	var routes []extension.ExtensionRoute

	if d.referrersEnabled {
		routes = append(routes, extension.ExtensionRoute{
			Namespace: namespaceName,
			Extension: extensionName,
			Component: referrersComponentName,
			Descriptor: v2.RouteDescriptor{
				Entity:      "Referrers",
				Description: "returns all referrers for a given digest",
				Methods: []v2.MethodDescriptor{
					{
						Method:      "GET",
						Description: "get all referrers for the given digest",
					},
				},
			},
			Dispatcher: d.referrersDispatcher,
		})
	}

	return routes
}

// GetRegistryRoutes returns a list of extension routes scoped at a registry level
// There are no registry scoped routes exposed by this namespace
func (d *orasNamespace) GetRegistryRoutes() []extension.ExtensionRoute {
	return nil
}

// GetNamespaceName returns the name associated with the namespace
func (d *orasNamespace) GetNamespaceName() string {
	return namespaceName
}

// GetNamespaceUrl returns the url link to the documentation where the namespace's extension and endpoints are defined
func (d *orasNamespace) GetNamespaceUrl() string {
	return namespaceUrl
}

// GetNamespaceDescription returns the description associated with the namespace
func (d *orasNamespace) GetNamespaceDescription() string {
	return namespaceDescription
}

func (o *orasNamespace) referrersDispatcher(extCtx *extension.ExtensionContext, r *http.Request) http.Handler {

	handler := &referrersHandler{
		storageDriver: o.storageDriver,
		extContext:    extCtx,
	}
	q := r.URL.Query()
	if dgstStr := q.Get("digest"); dgstStr == "" {
		dcontext.GetLogger(extCtx).Errorf("digest not available")
	} else if d, err := digest.Parse(dgstStr); err != nil {
		dcontext.GetLogger(extCtx).Errorf("error parsing digest=%q: %v", dgstStr, err)
	} else {
		handler.Digest = d
	}

	mhandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(handler.getReferrers),
	}

	return mhandler
}
