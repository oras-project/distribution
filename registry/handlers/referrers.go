package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"path"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/manifest/ociartifact"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// referrersDispatcher takes the request context and builds the
// appropriate handler for handling referrers requests.
func referrersDispatcher(ctx *Context, r *http.Request) http.Handler {
	dgst, err := getDigest(ctx)
	if err != nil {
		if err == errDigestNotAvailable {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx.Errors = append(ctx.Errors, v2.ErrorCodeDigestInvalid.WithDetail(err))
			})
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx.Errors = append(ctx.Errors, v2.ErrorCodeDigestInvalid.WithDetail(err))
		})
	}

	referrersHandler := &referrersHandler{
		Context: ctx,
		Digest:  dgst,
	}
	return handlers.MethodHandler{
		"GET": http.HandlerFunc(referrersHandler.GetReferrers),
	}
}

// referrersHandler handles http operations on referrers.
type referrersHandler struct {
	*Context
	Digest digest.Digest
}

// GetReferrers fetches the list of referrers as an image index from the storage.
func (h *referrersHandler) GetReferrers(w http.ResponseWriter, r *http.Request) {
	dcontext.GetLogger(h).Debug("GetReferrers")

	if h.Digest == "" {
		h.Errors = append(h.Errors, v2.ErrorCodeManifestUnknown.WithDetail("digest not specified"))
		return
	}

	var annotations map[string]string
	var artifactTypeFilter string
	if artifactTypeFilter = r.URL.Query().Get("artifactType"); artifactTypeFilter != "" {
		annotations = map[string]string{
			v1.AnnotationReferrersFiltersApplied: "artifactType",
		}
	}
	referrers, err := h.generateReferrersList(h, h.Digest, artifactTypeFilter)
	if err != nil {
		if _, ok := err.(distribution.ErrManifestUnknownRevision); ok {
			h.Errors = append(h.Errors, v2.ErrorCodeManifestUnknown.WithDetail(err))
		} else {
			h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	response := v1.Index{
		Versioned:   specs.Versioned{SchemaVersion: 2},
		MediaType:   v1.MediaTypeImageIndex,
		Manifests:   referrers,
		Annotations: annotations,
	}

	w.Header().Set("Content-Type", v1.MediaTypeImageIndex)
	enc := json.NewEncoder(w)
	if err = enc.Encode(response); err != nil {
		h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

func (h *referrersHandler) generateReferrersList(ctx context.Context, subjectDigest digest.Digest, artifactType string) ([]v1.Descriptor, error) {
	dcontext.GetLogger(ctx).Debug("(*referrersHandler).generateReferrersList")
	repo := h.Repository
	manifests, err := repo.Manifests(ctx)
	if err != nil {
		return nil, err
	}
	blobStatter := h.registry.BlobStatter()
	rootPath := storage.GetReferrersSearchPath(repo.Named().Name(), subjectDigest)
	var referrers []v1.Descriptor
	err = enumerateReferrerLinks(ctx,
		rootPath,
		h.driver,
		blobStatter,
		func(referrerDigest digest.Digest) error {
			man, err := manifests.Get(ctx, referrerDigest)
			if err != nil {
				return err
			}
			artifactManifest, ok := man.(*ociartifact.DeserializedManifest)
			if !ok {
				// The PUT handler would guard against this situation. Skip this manifest.
				return nil
			}
			extractedArtifactType := artifactManifest.ArtifactType
			// filtering by artifact type or bypass if no artifact type specified
			if artifactType == "" || extractedArtifactType == artifactType {
				desc, err := blobStatter.Stat(ctx, referrerDigest)
				if err != nil {
					return err
				}
				desc.MediaType, _, _ = man.Payload()
				artifactDesc := v1.Descriptor{
					MediaType:    desc.MediaType,
					Size:         desc.Size,
					Digest:       desc.Digest,
					ArtifactType: extractedArtifactType,
					Annotations:  artifactManifest.Annotations,
				}
				referrers = append(referrers, artifactDesc)
			}
			return nil
		})
	if err != nil {
		switch err.(type) {
		case driver.PathNotFoundError:
			return nil, nil
		}
		return nil, err
	}
	return referrers, nil
}

func enumerateReferrerLinks(ctx context.Context,
	rootPath string,
	stDriver driver.StorageDriver,
	blobstatter distribution.BlobStatter,
	ingestor func(digest digest.Digest) error) error {

	return stDriver.Walk(ctx, rootPath, func(fileInfo driver.FileInfo) error {
		// exit early if directory...
		if fileInfo.IsDir() {
			return nil
		}
		filePath := fileInfo.Path()

		// check if it's a link
		_, fileName := path.Split(filePath)
		if fileName != "link" {
			return nil
		}

		// read the digest found in link
		digest, err := readlink(ctx, filePath, stDriver)
		if err != nil {
			return err
		}

		// ensure this conforms to the linkPathFns
		_, err = blobstatter.Stat(ctx, digest)
		if err != nil {
			// we expect this error to occur so we move on
			if err == distribution.ErrBlobUnknown {
				return nil
			}
			return err
		}

		return ingestor(digest)
	})
}

func readlink(ctx context.Context, path string, stDriver driver.StorageDriver) (digest.Digest, error) {
	content, err := stDriver.GetContent(ctx, path)
	if err != nil {
		return "", err
	}

	return digest.Parse(string(content))
}
