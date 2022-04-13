package oci

import (
	"encoding/json"
	"net/http"

	"github.com/distribution/distribution/v3/registry/api/errcode"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage/driver"
)

type ociExtension struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
}

type discoverGetAPIResponse struct {
	Name       string         `json:"name"`
	Extensions []ociExtension `json:"extensions"`
}

// extensionHandler handles requests for manifests under a manifest name.
type extensionHandler struct {
	*extension.Context
	storageDriver driver.StorageDriver
}

func (th *extensionHandler) getExtensions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json")

	registered := extension.EnumerateRegistered()

	extensions := make([]ociExtension, len(registered))
	for _, e := range registered {
		extensions = append(extensions, ociExtension{Name: e.Name, Description: e.Description, Url: e.Path})
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(discoverGetAPIResponse{
		Name:       th.Repository.Named().Name(),
		Extensions: extensions,
	}); err != nil {
		th.Errors = append(th.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
