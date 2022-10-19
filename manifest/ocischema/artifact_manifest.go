package ocischema

import (
	"encoding/json"

	"github.com/distribution/distribution/v3"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

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
	references := make([]distribution.Descriptor, 0, 1+len(am.Blobs))
	references = append(references, am.Blobs...)
	references = append(references, am.Subject)
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
func ArtifactManifestFromStruct(am ArtifactManifest) (*DeserializedArtifactManifest, error) {
	var deserialized DeserializedArtifactManifest
	deserialized.ArtifactManifest = am

	var err error
	deserialized.canonical, err = json.MarshalIndent(&am, "", "   ")
	return &deserialized, err
}

// Payload returns the raw content of the artifact manifest. The contents can be used to
// calculate the content identifier.
func (am DeserializedArtifactManifest) Payload() (string, []byte, error) {
	return v1.MediaTypeArtifactManifest, am.canonical, nil
}
