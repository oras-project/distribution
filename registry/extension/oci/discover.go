package oci

import (
	"encoding/json"
	"net/http"

	"github.com/distribution/distribution/v3/registry/api/errcode"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage/driver"
)

type discoverGetAPIResponse struct {
	Name       string                         `json:"name"`
	Extensions []extension.EnumerateExtension `json:"extensions"`
}

// extensionHandler handles requests for manifests under a manifest name.
type extensionHandler struct {
	*extension.Context
	storageDriver driver.StorageDriver
}

func (th *extensionHandler) getExtensions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json")

	// get list of extension information seperated at the namespace level
	enumeratedExtensions := extension.EnumerateRegistered(r.Context())

	// remove the oci extension so it's not returned by discover
	for i, e := range enumeratedExtensions {
		if e.Name == namespaceName {
			enumeratedExtensions = append(enumeratedExtensions[:i], enumeratedExtensions[i+1:]...)
		}
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(discoverGetAPIResponse{
		Name:       th.Repository.Named().Name(),
		Extensions: enumeratedExtensions,
	}); err != nil {
		th.Errors = append(th.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
