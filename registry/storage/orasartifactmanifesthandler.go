package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/manifest/orasartifact"
	"github.com/opencontainers/go-digest"
)

// orasArtifactManifestHandler is a ManifestHandler that covers ORAS Artifacts.
type orasArtifactManifestHandler struct {
	repository     distribution.Repository
	blobStore      distribution.BlobStore
	ctx            context.Context
	referrersStore referrersStoreFunc
}

func (oamh *orasArtifactManifestHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(oamh.ctx).Debug("(*orasArtifactManifestHandler).Unmarshal")

	dm := &orasartifact.DeserializedManifest{}
	if err := dm.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return dm, nil
}

func (ah *orasArtifactManifestHandler) Put(ctx context.Context, man distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(ah.ctx).Debug("(*orasArtifactManifestHandler).Put")

	da, ok := man.(*orasartifact.DeserializedManifest)
	if !ok {
		return "", fmt.Errorf("wrong type put to orasArtifactManifestHandler: %T", man)
	}

	if err := ah.verifyManifest(ah.ctx, *da, skipDependencyVerification); err != nil {
		return "", err
	}

	mt, payload, err := da.Payload()
	if err != nil {
		return "", err
	}

	revision, err := ah.blobStore.Put(ctx, mt, payload)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error putting payload into blobstore: %v", err)
		return "", err
	}

	err = ah.indexReferrers(ctx, *da, revision.Digest)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error indexing referrers: %v", err)
		return "", err
	}

	return revision.Digest, nil
}

// verifyManifest ensures that the manifest content is valid from the
// perspective of the registry. As a policy, the registry only tries to
// store valid content, leaving trust policies of that content up to
// consumers.
func (oamh *orasArtifactManifestHandler) verifyManifest(ctx context.Context, dm orasartifact.DeserializedManifest, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if dm.ArtifactType() == "" {
		errs = append(errs, distribution.ErrManifestVerification{errors.New("artifactType invalid")})
	}

	if !skipDependencyVerification {
		bs := oamh.repository.Blobs(ctx)

		// All references must exist.
		for _, blobDesc := range dm.References() {
			desc, err := bs.Stat(ctx, blobDesc.Digest)
			if err != nil && err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
			}
			if err != nil || desc.Digest == "" {
				// On error here, we always append unknown blob errors.
				errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: blobDesc.Digest})
			}
		}

		ms, err := oamh.repository.Manifests(ctx)
		if err != nil {
			return err
		}

		// Validate subject manifest.
		subject := dm.Subject()
		exists, err := ms.Exists(ctx, subject.Digest)
		if !exists || err == distribution.ErrBlobUnknown {
			errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: subject.Digest})
		} else if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// indexReferrers indexes the subject of the given revision in its referrers index store.
func (oamh *orasArtifactManifestHandler) indexReferrers(ctx context.Context, dm orasartifact.DeserializedManifest, revision digest.Digest) error {
	artifactType := dm.ArtifactType()
	subject := dm.Subject()

	if err := oamh.referrersStore(ctx, subject.Digest, artifactType).linkBlob(ctx, distribution.Descriptor{Digest: revision}); err != nil {
		return err
	}

	return nil
}
