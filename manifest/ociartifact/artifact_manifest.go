package ociartifact

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	artifactFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		if err := validateArtifactManifest(b); err != nil {
			return nil, distribution.Descriptor{}, err
		}
		m := new(DeserializedArtifactManifest)
		err := m.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return m, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: v1.MediaTypeArtifactManifest}, err
	}
	err := distribution.RegisterManifestSchema(v1.MediaTypeArtifactManifest, artifactFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register artifact manifest: %s", err))
	}
}

// ArtifactManifest defines an ocischema artifact manifest.
type ArtifactManifest struct {
	// MediaType must be application/vnd.oci.artifact.manifest.v1+json.
	MediaType string `json:"mediaType"`

	// ArtifactType contains the mediaType of the referenced artifact.
	// If defined, the value MUST comply with RFC 6838, including the naming
	// requirements in its section 4.2, and MAY be registered with IANA.
	ArtifactType string `json:"artifactType,omitempty"`

	// Blobs lists descriptors for the blobs referenced by the artifact.
	Blobs []distribution.Descriptor `json:"blobs,omitempty"`

	// Subject specifies the descriptor of another manifest. This value is
	// used by the referrers API.
	Subject distribution.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the artifact manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// References returns the descriptors of this artifact manifest references.
func (am ArtifactManifest) References() []distribution.Descriptor {
	var references []distribution.Descriptor
	references = append(references, am.Blobs...)
	// if Subject exists, append it to references, this part needs more design
	if am.Subject.Digest != "" {
		references = append(references, am.Subject)
	}
	return references
}

// DeserializedArtifactManifest wraps ArtifactManifest with a copy of the original JSON.
// It satisfies the distribution.Manifest interface.
type DeserializedArtifactManifest struct {
	ArtifactManifest

	// canonical is the canonical byte representation of the ArtifactManifest.
	canonical []byte
}

// ArtifactManifestFromStruct takes an ArtifactManifest structure, marshals it to JSON, and returns a
// DeserializedArtifactManifest which contains the manifest and its JSON representation.
func ArtifactManifestFromStruct(m ArtifactManifest) (*DeserializedArtifactManifest, error) {
	var deserialized DeserializedArtifactManifest
	deserialized.ArtifactManifest = m

	var err error
	deserialized.canonical, err = json.MarshalIndent(&m, "", "   ")
	return &deserialized, err
}

// UnmarshalJSON populates a new ArtifactManifest struct from JSON data.
func (m *DeserializedArtifactManifest) UnmarshalJSON(b []byte) error {
	m.canonical = make([]byte, len(b))
	// store manifest in canonical
	copy(m.canonical, b)

	// Unmarshal canonical JSON into an ArtifactManifest object
	var manifest ArtifactManifest
	if err := json.Unmarshal(m.canonical, &manifest); err != nil {
		return err
	}

	if manifest.MediaType != v1.MediaTypeArtifactManifest {
		return fmt.Errorf("mediaType in manifest should be '%s' not '%s'",
			v1.MediaTypeArtifactManifest, manifest.MediaType)
	}

	m.ArtifactManifest = manifest

	return nil
}

// MarshalJSON returns the contents of canonical. If canonical is empty,
// marshals the inner contents.
func (m *DeserializedArtifactManifest) MarshalJSON() ([]byte, error) {
	if len(m.canonical) > 0 {
		return m.canonical, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedArtifactManifest")
}

// Payload returns the raw content of the artifact manifest. The contents can be used to
// calculate the content identifier.
func (m DeserializedArtifactManifest) Payload() (string, []byte, error) {
	return v1.MediaTypeArtifactManifest, m.canonical, nil
}

// unknownDocument represents a manifest, manifest list, or index that has not
// yet been validated
type unknownDocument struct {
	Config    interface{} `json:"config,omitempty"`
	Manifests interface{} `json:"manifests,omitempty"`
}

// validateArtifactManifest returns an error if the byte slice is invalid JSON or if it
// contains fields that belong to an index or an image manifest
func validateArtifactManifest(b []byte) error {
	var doc unknownDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if doc.Config != nil {
		return errors.New("oci artifact manifest: expected artifact manifest but found image manifest")
	}
	if doc.Manifests != nil {
		return errors.New("oci artifact manifest: expected artifact manifest but found index")
	}
	return nil
}
