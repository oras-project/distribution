package oras

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest"
	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/distribution/distribution/v3/reference"
	"github.com/distribution/distribution/v3/registry/extension"
	storage "github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/opencontainers/go-digest"
	orasartifacts "github.com/oras-project/artifacts-spec/specs-go/v1"
)

func createRegistry(t *testing.T, driver driver.StorageDriver, options ...storage.RegistryOption) distribution.Namespace {
	ctx := context.Background()
	options = append([]storage.RegistryOption{storage.EnableDelete}, options...)
	extensionConfig := OrasOptions{
		ArtifactsExtComponents: []string{"referrers"},
	}
	ns, err := extension.Get(ctx, "oras", driver, extensionConfig)
	if err != nil {
		t.Fatalf("unable to configure extension namespace (%s): %s", "oras", err)
	}
	options = append(options, storage.AddExtendedStorage(ns))
	registry, err := storage.NewRegistry(ctx, driver, options...)
	if err != nil {
		t.Fatalf("failed to construct namespace")
	}
	return registry
}

func makeRepository(t *testing.T, registry distribution.Namespace, name string) distribution.Repository {
	ctx := context.Background()
	named, err := reference.WithName(name)
	if err != nil {
		t.Fatalf("failed to parse name %s:  %v", name, err)
	}

	repo, err := registry.Repository(ctx, named)
	if err != nil {
		t.Fatalf("failed to construct repository: %v", err)
	}
	return repo
}

func makeManifestService(t *testing.T, repository distribution.Repository) distribution.ManifestService {
	ctx := context.Background()

	manifestService, err := repository.Manifests(ctx)
	if err != nil {
		t.Fatalf("failed to construct manifest store: %v", err)
	}
	return manifestService
}

func TestVerifyArtifactManifestPut(t *testing.T) {
	ctx := context.Background()
	inmemoryDriver := inmemory.New()
	registry := createRegistry(t, inmemoryDriver)
	repo := makeRepository(t, registry, "test")
	manifestService := makeManifestService(t, repo)

	artifactBlob, err := repo.Blobs(ctx).Put(ctx, orasartifacts.MediaTypeDescriptor, nil)
	if err != nil {
		t.Fatal(err)
	}

	config, err := repo.Blobs(ctx).Put(ctx, schema2.MediaTypeImageConfig, nil)
	if err != nil {
		t.Fatal(err)
	}

	layer, err := repo.Blobs(ctx).Put(ctx, schema2.MediaTypeLayer, nil)
	if err != nil {
		t.Fatal(err)
	}

	subjectManifest := schema2.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 2,
			MediaType:     schema2.MediaTypeManifest,
		},
		Config: config,
		Layers: []distribution.Descriptor{
			layer,
		},
	}

	dm, err := schema2.FromStruct(subjectManifest)
	if err != nil {
		t.Fatalf("failed to marshal subject manifest: %v", err)
	}
	_, dmPayload, err := dm.Payload()
	if err != nil {
		t.Fatalf("failed to get subject manifest payload: %v", err)
	}

	dg, err := manifestService.Put(ctx, dm)
	if err != nil {
		t.Fatalf("failed to put subject manifest with err: %v", err)
	}

	artifactBlobDescriptor := orasartifacts.Descriptor{
		MediaType: artifactBlob.MediaType,
		Digest:    artifactBlob.Digest,
		Size:      artifactBlob.Size,
	}

	template := Manifest{
		inner: orasartifacts.Manifest{
			MediaType:    orasartifacts.MediaTypeArtifactManifest,
			ArtifactType: "test_artifactType",
			Blobs: []orasartifacts.Descriptor{
				artifactBlobDescriptor,
			},
			Subject: &orasartifacts.Descriptor{
				MediaType: dm.MediaType,
				Size:      int64(len(dmPayload)),
				Digest:    dg,
			},
			Annotations: map[string]string{
				createAnnotationName: "2022-04-22T17:03:05-07:00",
			},
		},
	}

	type testcase struct {
		MediaType    string
		ArtifactType string
		Blobs        []orasartifacts.Descriptor
		Subject      *orasartifacts.Descriptor
		Annotations  map[string]string
		Err          error
	}

	cases := []testcase{
		{
			orasartifacts.MediaTypeArtifactManifest,
			template.inner.ArtifactType,
			template.inner.Blobs,
			template.inner.Subject,
			template.Annotations(),
			nil,
		},
		// non oras artifact manifest media type
		{
			"wrongMediaType",
			template.inner.ArtifactType,
			template.inner.Blobs,
			template.inner.Subject,
			template.Annotations(),
			errInvalidMediaType,
		},
		// empty artifactType
		{
			orasartifacts.MediaTypeArtifactManifest,
			"",
			template.inner.Blobs,
			template.inner.Subject,
			template.Annotations(),
			errInvalidArtifactType,
		},
		// invalid subject
		{
			orasartifacts.MediaTypeArtifactManifest,
			template.inner.ArtifactType,
			template.inner.Blobs,
			&orasartifacts.Descriptor{
				MediaType: dm.MediaType,
				Size:      int64(len(dmPayload)),
				Digest:    digest.FromString("sha256:invalid"),
			},
			template.Annotations(),
			distribution.ErrManifestBlobUnknown{Digest: digest.FromString("sha256:invalid")},
		},
		// invalid created annotation
		{
			orasartifacts.MediaTypeArtifactManifest,
			template.inner.ArtifactType,
			template.inner.Blobs,
			template.inner.Subject,
			map[string]string{
				createAnnotationName: "invalid_timestamp",
			},
			errInvalidCreatedAnnotation,
		},
		// invalid blob
		{
			orasartifacts.MediaTypeArtifactManifest,
			template.inner.ArtifactType,
			[]orasartifacts.Descriptor{
				{
					MediaType: artifactBlob.MediaType,
					Digest:    digest.FromString("sha256:invalid_blob_digest"),
					Size:      artifactBlob.Size,
				},
			},
			template.inner.Subject,
			template.Annotations(),
			distribution.ErrManifestBlobUnknown{Digest: digest.FromString("sha256:invalid_blob_digest")},
		},
	}

	for _, c := range cases {
		manifest := Manifest{
			inner: orasartifacts.Manifest{
				MediaType:    c.MediaType,
				ArtifactType: c.ArtifactType,
				Blobs:        c.Blobs,
				Subject:      c.Subject,
				Annotations:  c.Annotations,
			},
		}

		marshalledManifest, err := json.Marshal(manifest.inner)
		if err != nil {
			t.Fatalf("failed to marshal manifest: %v", err)
		}

		_, err = manifestService.Put(ctx, &DeserializedManifest{
			Manifest: manifest,
			raw:      marshalledManifest,
		})
		if verr, ok := err.(distribution.ErrManifestVerification); ok {
			err = verr[0]
		}
		if err != c.Err {
			t.Errorf("expected %v, got %v", c.Err, err)
		}
	}
}
