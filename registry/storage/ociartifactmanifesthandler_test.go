package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest/ociartifact"
	"github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestVerifyOCIArtifactManifestBlobsAndSubject(t *testing.T) {
	ctx := context.Background()
	inmemoryDriver := inmemory.New()
	registry := createRegistry(t, inmemoryDriver)
	repo := makeRepository(t, registry, strings.ToLower(t.Name()))
	manifestService := makeManifestService(t, repo)

	subject, err := repo.Blobs(ctx).Put(ctx, v1.MediaTypeImageManifest, nil)
	if err != nil {
		t.Fatal(err)
	}

	blob, err := repo.Blobs(ctx).Put(ctx, v1.MediaTypeImageLayer, nil)
	if err != nil {
		t.Fatal(err)
	}

	template := ociartifact.Manifest{
		MediaType: v1.MediaTypeArtifactManifest,
	}

	checkFn := func(m ociartifact.Manifest, rerr error) {
		dm, err := ociartifact.FromStruct(m)
		if err != nil {
			t.Error(err)
			return
		}
		_, err = manifestService.Put(ctx, dm)
		if verr, ok := err.(distribution.ErrManifestVerification); ok {
			// Extract the first error
			if len(verr) == 2 {
				if _, ok = verr[1].(distribution.ErrManifestBlobUnknown); ok {
					err = verr[0]
				}
			} else if len(verr) == 1 {
				err = verr[0]
			}
		}
		if err != rerr {
			t.Errorf("%#v: expected %v, got %v", m, rerr, err)
		}
	}

	type testcase struct {
		Desc distribution.Descriptor
		Err  error
	}

	layercases := []testcase{
		// normal blob with media type v1.MediaTypeImageLayer
		{
			blob,
			nil,
		},
		// blob with empty media type but valid digest
		{
			distribution.Descriptor{Digest: blob.Digest},
			nil,
		},
		// blob with invalid digest
		{
			distribution.Descriptor{Digest: digest.Digest("invalid")},
			digest.ErrDigestInvalidFormat,
		},
		// empty descriptor
		{
			distribution.Descriptor{},
			digest.ErrDigestInvalidFormat,
		},
	}

	for _, c := range layercases {
		m := template
		m.Subject = &subject
		m.Blobs = []distribution.Descriptor{c.Desc}
		checkFn(m, c.Err)
	}

	subjectcases := []testcase{
		// normal subject
		{
			subject,
			nil,
		},
		// subject with invalid digest
		{
			distribution.Descriptor{Digest: digest.Digest("invalid")},
			digest.ErrDigestInvalidFormat,
		},
	}

	for _, c := range subjectcases {
		m := template
		m.Subject = &c.Desc
		m.Blobs = []distribution.Descriptor{blob}
		checkFn(m, c.Err)
	}
}
