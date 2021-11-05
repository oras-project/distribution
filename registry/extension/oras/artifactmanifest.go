package oras

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	"github.com/opencontainers/go-digest"
	v1 "github.com/oras-project/artifacts-spec/specs-go/v1"
)

func init() {
	unmarshalFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		d := new(DeserializedManifest)
		err := d.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return d, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: v1.MediaTypeArtifactManifest}, err
	}
	err := distribution.RegisterManifestSchema(v1.MediaTypeArtifactManifest, unmarshalFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register ORAS artifact manifest: %s", err))
	}
}

// Manifest describes ORAS artifact manifests.
type Manifest struct {
	inner v1.Manifest
}

// ArtifactType returns the artifactType of this ORAS artifact.
func (a Manifest) ArtifactType() string {
	return a.inner.ArtifactType
}

// References returns the distribution descriptors for the referenced blobs.
func (a Manifest) References() []distribution.Descriptor {
	blobs := make([]distribution.Descriptor, len(a.inner.Blobs))
	for i := range a.inner.Blobs {
		blobs[i] = distribution.Descriptor{
			MediaType: a.inner.Blobs[i].MediaType,
			Digest:    a.inner.Blobs[i].Digest,
			Size:      a.inner.Blobs[i].Size,
		}
	}
	return blobs
}

// Subject returns the the subject manifest this artifact references.
func (a Manifest) Subject() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: a.inner.Subject.MediaType,
		Digest:    a.inner.Subject.Digest,
		Size:      a.inner.Subject.Size,
	}
}

// DeserializedManifest wraps Manifest with a copy of the original JSON data.
type DeserializedManifest struct {
	Manifest

	// raw is the raw byte representation of the ORAS artifact.
	raw []byte
}

// UnmarshalJSON populates a new Manifest struct from JSON data.
func (d *DeserializedManifest) UnmarshalJSON(b []byte) error {
	d.raw = make([]byte, len(b))
	copy(d.raw, b)

	var man v1.Manifest
	if err := json.Unmarshal(d.raw, &man); err != nil {
		return err
	}
	if man.ArtifactType == "" {
		return errors.New("artifactType cannot be empty")
	}
	d.inner = man

	return nil
}

// MarshalJSON returns the raw content.
func (d *DeserializedManifest) MarshalJSON() ([]byte, error) {
	if len(d.raw) > 0 {
		return d.raw, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedManifest")
}

// Payload returns the raw content of the Artifact. The contents can be
// used to calculate the content identifier.
func (d DeserializedManifest) Payload() (string, []byte, error) {
	// NOTE: This is a hack. The media type should be read from storage.
	return v1.MediaTypeArtifactManifest, d.raw, nil
}
