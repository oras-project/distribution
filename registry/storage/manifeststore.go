package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/manifest"
	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/distribution/distribution/v3/manifest/ocischema"
	"github.com/distribution/distribution/v3/manifest/orasartifact"
	"github.com/distribution/distribution/v3/manifest/schema1"
	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	orasartifactv1 "github.com/oras-project/artifacts-spec/specs-go/v1"
)

// referrersStoreFunc describes a function that returns the referrers store
// for the given manifest of the given artifactType.
// A referrers store provides links to referrer manifests.
type referrersStoreFunc func(ctx context.Context, revision digest.Digest, artifactType string) *linkedBlobStore

// A ManifestHandler gets and puts manifests of a particular type.
type ManifestHandler interface {
	// Unmarshal unmarshals the manifest from a byte slice.
	Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error)

	// Put creates or updates the given manifest returning the manifest digest.
	Put(ctx context.Context, manifest distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error)
}

// SkipLayerVerification allows a manifest to be Put before its
// layers are on the filesystem
func SkipLayerVerification() distribution.ManifestServiceOption {
	return skipLayerOption{}
}

type skipLayerOption struct{}

func (o skipLayerOption) Apply(m distribution.ManifestService) error {
	if ms, ok := m.(*manifestStore); ok {
		ms.skipDependencyVerification = true
		return nil
	}
	return fmt.Errorf("skip layer verification only valid for manifestStore")
}

type manifestStore struct {
	repository *repository
	blobStore  *linkedBlobStore
	ctx        context.Context

	skipDependencyVerification bool

	schema1Handler      ManifestHandler
	schema2Handler      ManifestHandler
	ocischemaHandler    ManifestHandler
	manifestListHandler ManifestHandler
	orasArtifactHandler ManifestHandler

	referrersStore referrersStoreFunc
}

var _ distribution.ManifestService = &manifestStore{}

func (ms *manifestStore) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*manifestStore).Exists")

	_, err := ms.blobStore.Stat(ms.ctx, dgst)
	if err != nil {
		if err == distribution.ErrBlobUnknown {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (ms *manifestStore) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*manifestStore).Get")

	// TODO(stevvooe): Need to check descriptor from above to ensure that the
	// mediatype is as we expect for the manifest store.

	content, err := ms.blobStore.Get(ctx, dgst)
	if err != nil {
		if err == distribution.ErrBlobUnknown {
			return nil, distribution.ErrManifestUnknownRevision{
				Name:     ms.repository.Named().Name(),
				Revision: dgst,
			}
		}

		return nil, err
	}

	var versioned manifest.Versioned
	if err = json.Unmarshal(content, &versioned); err != nil {
		return nil, err
	}

	switch versioned.SchemaVersion {
	case 1:
		return ms.schema1Handler.Unmarshal(ctx, dgst, content)
	case 2:
		// This can be an image manifest or a manifest list
		switch versioned.MediaType {
		case schema2.MediaTypeManifest:
			return ms.schema2Handler.Unmarshal(ctx, dgst, content)
		case v1.MediaTypeImageManifest:
			return ms.ocischemaHandler.Unmarshal(ctx, dgst, content)
		case manifestlist.MediaTypeManifestList, v1.MediaTypeImageIndex:
			return ms.manifestListHandler.Unmarshal(ctx, dgst, content)
		case "":
			// OCI image or image index - no media type in the content

			// First see if it looks like an image index
			res, err := ms.manifestListHandler.Unmarshal(ctx, dgst, content)
			resIndex := res.(*manifestlist.DeserializedManifestList)
			if err == nil && resIndex.Manifests != nil {
				return resIndex, nil
			}

			// Otherwise, assume it must be an image manifest
			return ms.ocischemaHandler.Unmarshal(ctx, dgst, content)
		default:
			return nil, distribution.ErrManifestVerification{fmt.Errorf("unrecognized manifest content type %s", versioned.MediaType)}
		}
	default:
		// Assume it's an ORAS artifact manifest.
		return ms.orasArtifactHandler.Unmarshal(ctx, dgst, content)
	}
}

func (ms *manifestStore) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*manifestStore).Put")

	switch manifest.(type) {
	case *schema1.SignedManifest:
		return ms.schema1Handler.Put(ctx, manifest, ms.skipDependencyVerification)
	case *schema2.DeserializedManifest:
		return ms.schema2Handler.Put(ctx, manifest, ms.skipDependencyVerification)
	case *ocischema.DeserializedManifest:
		return ms.ocischemaHandler.Put(ctx, manifest, ms.skipDependencyVerification)
	case *manifestlist.DeserializedManifestList:
		return ms.manifestListHandler.Put(ctx, manifest, ms.skipDependencyVerification)
	case *orasartifact.DeserializedManifest:
		return ms.orasArtifactHandler.Put(ctx, manifest, ms.skipDependencyVerification)
	}

	return "", fmt.Errorf("unrecognized manifest type %T", manifest)
}

// Referrers returns referrer manifests filtered by the given referrerType.
func (ms *manifestStore) Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]orasartifactv1.Descriptor, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*manifestStore).Referrers")

	var referrers []orasartifactv1.Descriptor

	err := ms.referrersStore(ctx, revision, referrerType).Enumerate(ctx, func(referrerRevision digest.Digest) error {
		man, err := ms.Get(ctx, referrerRevision)
		if err != nil {
			return err
		}

		orasArtifactMan, ok := man.(*orasartifact.DeserializedManifest)
		if !ok {
			// The PUT handler would guard against this situation. Skip this manifest.
			return nil
		}

		desc, err := ms.blobStore.Stat(ctx, referrerRevision)
		if err != nil {
			return err
		}
		desc.MediaType, _, _ = man.Payload()
		referrers = append(referrers, orasartifactv1.Descriptor{
			MediaType:    desc.MediaType,
			Size:         desc.Size,
			Digest:       desc.Digest,
			ArtifactType: orasArtifactMan.ArtifactType(),
		})
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

// Delete removes the revision of the specified manifest.
func (ms *manifestStore) Delete(ctx context.Context, dgst digest.Digest) error {
	dcontext.GetLogger(ms.ctx).Debug("(*manifestStore).Delete")
	return ms.blobStore.Delete(ctx, dgst)
}

func (ms *manifestStore) Enumerate(ctx context.Context, ingester func(digest.Digest) error) error {
	err := ms.blobStore.Enumerate(ctx, func(dgst digest.Digest) error {
		err := ingester(dgst)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}
