package storage

import (
	"context"
	"fmt"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/manifest/ociartifact"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// ociArtifactManifestHandler is a ManifestHandler that covers ocischema manifests.
type ociArtifactManifestHandler struct {
	repository distribution.Repository
	blobStore  distribution.BlobStore
	ctx        context.Context
}

var _ ManifestHandler = &ociArtifactManifestHandler{}

func (ms *ociArtifactManifestHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*ociArtifactManifestHandler).Unmarshal")

	m := &ociartifact.DeserializedArtifactManifest{}
	if err := m.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return m, nil
}

func (ms *ociArtifactManifestHandler) Put(ctx context.Context, manifest distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*ociArtifactManifestHandler).Put")

	m, ok := manifest.(*ociartifact.DeserializedArtifactManifest)
	if !ok {
		return "", fmt.Errorf("non-oci artifact manifest put to ociArtifactManifestHandler: %T", manifest)
	}

	if err := ms.verifyArtifactManifest(ms.ctx, *m, skipDependencyVerification); err != nil {
		return "", err
	}

	mt, payload, err := m.Payload()
	if err != nil {
		return "", err
	}

	revision, err := ms.blobStore.Put(ctx, mt, payload)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error putting payload into blobstore: %v", err)
		return "", err
	}

	return revision.Digest, nil
}

// verifyArtifactManifest ensures that the manifest content is valid from the
// perspective of the registry. As a policy, the registry only tries to store
// valid content, leaving trust policies of that content up to consumers.
func (ms *ociArtifactManifestHandler) verifyArtifactManifest(ctx context.Context, mnfst ociartifact.DeserializedArtifactManifest, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if mnfst.MediaType != v1.MediaTypeArtifactManifest {
		return fmt.Errorf("unrecognized manifest media type %s", mnfst.MediaType)
	}

	if skipDependencyVerification {
		return nil
	}

	manifestService, err := ms.repository.Manifests(ctx)
	if err != nil {
		return err
	}

	for _, descriptor := range mnfst.References() {
		err := descriptor.Digest.Validate()
		if err != nil {
			errs = append(errs, err, distribution.ErrManifestBlobUnknown{Digest: descriptor.Digest})
			continue
		}
		exists, err := manifestService.Exists(ctx, descriptor.Digest)
		if err != nil || !exists {
			errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: descriptor.Digest})
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}
