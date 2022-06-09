package oras

import (
	"context"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/opencontainers/go-digest"
	artifactv1 "github.com/oras-project/artifacts-spec/specs-go/v1"
)

type ArtifactService interface {
	Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]artifactv1.Descriptor, error)
}

// referrersHandler handles http operations on manifest referrers.
type referrersHandler struct {
	extContext    *extension.Context
	storageDriver driver.StorageDriver

	// Digest is the target manifest's digest.
	Digest digest.Digest
}

type referrersSortedWrapper struct {
	createdAt  time.Time
	descriptor artifactv1.Descriptor
}

const createAnnotationName = "io.cncf.oras.artifact.created"
const createAnnotationTimestampFormat = time.RFC3339

func (h *referrersHandler) Referrers(ctx context.Context, revision digest.Digest, artifactType string) ([]artifactv1.Descriptor, error) {
	dcontext.GetLogger(ctx).Debug("(*manifestStore).Referrers")

	var referrersUnsorted []artifactv1.Descriptor
	var referrersSorted []artifactv1.Descriptor
	var referrersWrappers []referrersSortedWrapper

	repo := h.extContext.Repository
	manifests, err := repo.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	blobStatter := h.extContext.Registry.BlobStatter()
	rootPath := path.Join(referrersLinkPath(repo.Named().Name()), revision.Algorithm().String(), revision.Hex())
	err = storage.EnumerateReferrerLinks(ctx,
		rootPath,
		h.storageDriver,
		blobStatter,
		manifests,
		repo.Named().Name(),
		map[digest.Digest]struct{}{},
		map[digest.Digest]storage.ArtifactManifestDel{},
		func(ctx context.Context,
			referrerRevision digest.Digest,
			manifestService distribution.ManifestService,
			markSet map[digest.Digest]struct{},
			artifactManifestIndex map[digest.Digest]storage.ArtifactManifestDel,
			repoName string,
			storageDriver driver.StorageDriver,
			blobStatter distribution.BlobStatter) error {
			man, err := manifests.Get(ctx, referrerRevision)
			if err != nil {
				return err
			}

			ArtifactMan, ok := man.(*DeserializedManifest)
			if !ok {
				// The PUT handler would guard against this situation. Skip this manifest.
				return nil
			}

			extractedArtifactType := ArtifactMan.ArtifactType()

			// filtering by artifact type or bypass if no artifact type specified
			if artifactType == "" || extractedArtifactType == artifactType {
				desc, err := blobStatter.Stat(ctx, referrerRevision)
				if err != nil {
					return err
				}
				desc.MediaType, _, _ = man.Payload()
				artifactDesc := artifactv1.Descriptor{
					MediaType:    desc.MediaType,
					Size:         desc.Size,
					Digest:       desc.Digest,
					ArtifactType: extractedArtifactType,
				}

				if annotation, ok := ArtifactMan.Annotations()[createAnnotationName]; !ok {
					referrersUnsorted = append(referrersUnsorted, artifactDesc)
				} else {
					extractedTimestamp, err := time.Parse(createAnnotationTimestampFormat, annotation)
					if err != nil {
						return fmt.Errorf("failed to parse created annotation timestamp: %v", err)
					}
					referrersWrappers = append(referrersWrappers, referrersSortedWrapper{
						createdAt:  extractedTimestamp,
						descriptor: artifactDesc,
					})
				}
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

	// sort the list of descriptors that contain the created annotation
	sort.Slice(referrersWrappers, func(i, j int) bool {
		// most recent artifact first
		return referrersWrappers[i].createdAt.After(referrersWrappers[j].createdAt)
	})
	// extract the artifact descriptor from the sorting wrapper
	for _, wrapper := range referrersWrappers {
		referrersSorted = append(referrersSorted, wrapper.descriptor)
	}
	// append the descriptors, which don't have a created annotation, to the end
	referrersSorted = append(referrersSorted, referrersUnsorted...)
	return referrersSorted, nil
}