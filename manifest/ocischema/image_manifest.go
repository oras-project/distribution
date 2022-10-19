package ocischema

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// SchemaVersion provides a pre-initialized version structure for this
	// packages version of the manifest.
	SchemaVersion = manifest.Versioned{
		SchemaVersion: 2, // historical value here.. does not pertain to OCI or docker version
		MediaType:     v1.MediaTypeImageManifest,
	}
)

func init() {
	ociImageFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		if err := validateImageManifest(b); err != nil {
			return nil, distribution.Descriptor{}, err
		}
		m := new(DeserializedImageManifest)
		err := m.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return m, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: v1.MediaTypeImageManifest}, err
	}
	err := distribution.RegisterManifestSchema(v1.MediaTypeImageManifest, ociImageFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register image manifest: %s", err))
	}
}

// ImageManifest defines a ocischema image manifest.
type ImageManifest struct {
	manifest.Versioned

	// Config references the image configuration as a blob.
	Config distribution.Descriptor `json:"config"`

	// Layers lists descriptors for the layers referenced by the
	// configuration.
	Layers []distribution.Descriptor `json:"layers"`

	// Annotations contains arbitrary metadata for the image manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// References returns the descriptors of this manifests references.
func (m ImageManifest) References() []distribution.Descriptor {
	references := make([]distribution.Descriptor, 0, 1+len(m.Layers))
	references = append(references, m.Config)
	references = append(references, m.Layers...)
	return references
}

// Target returns the target of this manifest.
func (m ImageManifest) Target() distribution.Descriptor {
	return m.Config
}

// DeserializedImageManifest wraps ImageManifest with a copy of the original JSON.
// It satisfies the distribution.Manifest interface.
type DeserializedImageManifest struct {
	ImageManifest

	// canonical is the canonical byte representation of the ImageManifest.
	canonical []byte
}

// ImageManifestFromStruct takes an ImageManifest structure, marshals it to JSON, and returns a
// DeserializedImageManifest which contains the manifest and its JSON representation.
func ImageManifestFromStruct(m ImageManifest) (*DeserializedImageManifest, error) {
	var deserialized DeserializedImageManifest
	deserialized.ImageManifest = m

	var err error
	deserialized.canonical, err = json.MarshalIndent(&m, "", "   ")
	return &deserialized, err
}

// UnmarshalJSON populates a new ImageManifest struct from JSON data.
func (m *DeserializedImageManifest) UnmarshalJSON(b []byte) error {
	m.canonical = make([]byte, len(b))
	// store manifest in canonical
	copy(m.canonical, b)

	// Unmarshal canonical JSON into ImageManifest object
	var manifest ImageManifest
	if err := json.Unmarshal(m.canonical, &manifest); err != nil {
		return err
	}

	if manifest.MediaType != "" && manifest.MediaType != v1.MediaTypeImageManifest {
		return fmt.Errorf("if present, mediaType in image manifest should be '%s' not '%s'",
			v1.MediaTypeImageManifest, manifest.MediaType)
	}

	m.ImageManifest = manifest

	return nil
}

// MarshalJSON returns the contents of canonical. If canonical is empty,
// marshals the inner contents.
func (m *DeserializedImageManifest) MarshalJSON() ([]byte, error) {
	if len(m.canonical) > 0 {
		return m.canonical, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedImageManifest")
}

// Payload returns the raw content of the manifest. The contents can be used to
// calculate the content identifier.
func (m DeserializedImageManifest) Payload() (string, []byte, error) {
	return v1.MediaTypeImageManifest, m.canonical, nil
}

// unknownDocument represents a manifest, manifest list, or index that has not
// yet been validated
type unknownDocument struct {
	Manifests interface{} `json:"manifests,omitempty"`
}

// validateImageManifest returns an error if the byte slice is invalid JSON or if it
// contains fields that belong to a index
func validateImageManifest(b []byte) error {
	var doc unknownDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if doc.Manifests != nil {
		return errors.New("oci image manifest: expected image manifest but found index")
	}
	return nil
}
