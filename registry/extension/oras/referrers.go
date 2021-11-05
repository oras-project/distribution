package oras

import (
	"encoding/json"
	"net/http"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	orasartifacts "github.com/oras-project/artifacts-spec/specs-go/v1"
)

// referrersResponse describes the response body of the referrers API.
type referrersResponse struct {
	Referrers []orasartifacts.Descriptor `json:"references"`
}

func (h *referrersHandler) getReferrers(w http.ResponseWriter, r *http.Request) {
	dcontext.GetLogger(h.extContext).Debug("Get")

	// This can be empty
	artifactType := r.FormValue("artifactType")

	if h.Digest == "" {
		h.extContext.Errors = append(h.extContext.Errors, v2.ErrorCodeManifestUnknown.WithDetail("digest not specified"))
		return
	}

	referrers, err := h.Referrers(h.extContext, h.Digest, artifactType)
	if err != nil {
		if _, ok := err.(distribution.ErrManifestUnknownRevision); ok {
			h.extContext.Errors = append(h.extContext.Errors, v2.ErrorCodeManifestUnknown.WithDetail(err))
		} else {
			h.extContext.Errors = append(h.extContext.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	if referrers == nil {
		referrers = []orasartifacts.Descriptor{}
	}

	response := referrersResponse{
		Referrers: referrers,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err = enc.Encode(response); err != nil {
		h.extContext.Errors = append(h.extContext.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
