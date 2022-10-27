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
		m := new(DeserializedManifest)
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

// Manifest defines an ocischema artifact manifest.
type Manifest struct {
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
	Subject *distribution.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the artifact manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// References returns the descriptors of this artifact manifest references.
func (m Manifest) References() []distribution.Descriptor {
	var references []distribution.Descriptor
	references = append(references, m.Blobs...)
	if m.Subject != nil {
		references = append(references, *m.Subject)
	}
	return references
}

// DeserializedManifest wraps Manifest with a copy of the original JSON.
// It satisfies the distribution.Manifest interface.
type DeserializedManifest struct {
	Manifest

	// canonical is the canonical byte representation of the Manifest.
	canonical []byte
}

// FromStruct takes an Manifest structure, marshals it to JSON, and returns a
// DeserializedManifest which contains the manifest and its JSON representation.
func FromStruct(m Manifest) (*DeserializedManifest, error) {
	var deserialized DeserializedManifest
	deserialized.Manifest = m

	var err error
	deserialized.canonical, err = json.MarshalIndent(&m, "", "   ")
	return &deserialized, err
}

// UnmarshalJSON populates a new Manifest struct from JSON data.
func (m *DeserializedManifest) UnmarshalJSON(b []byte) error {
	m.canonical = make([]byte, len(b))
	// store manifest in canonical
	copy(m.canonical, b)

	// Unmarshal canonical JSON into an Manifest object
	var manifest Manifest
	if err := json.Unmarshal(m.canonical, &manifest); err != nil {
		return err
	}

	if manifest.MediaType != v1.MediaTypeArtifactManifest {
		return fmt.Errorf("mediaType in manifest should be '%s' not '%s'",
			v1.MediaTypeArtifactManifest, manifest.MediaType)
	}

	m.Manifest = manifest

	return nil
}

// MarshalJSON returns the contents of canonical. If canonical is empty,
// marshals the inner contents.
func (m *DeserializedManifest) MarshalJSON() ([]byte, error) {
	if len(m.canonical) > 0 {
		return m.canonical, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedManifest")
}

// Payload returns the raw content of the artifact manifest. The contents can be used to
// calculate the content identifier.
func (m DeserializedManifest) Payload() (string, []byte, error) {
	return v1.MediaTypeArtifactManifest, m.canonical, nil
}
