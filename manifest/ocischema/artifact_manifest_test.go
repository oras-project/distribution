package ocischema

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest/manifestlist"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Example showing an artifact manifest for an example SBOM referencing an image,
// taken from https://github.com/opencontainers/image-spec/blob/main/artifact.md.
var expectedArtifactManifestSerialization = []byte(`{
   "mediaType": "application/vnd.oci.artifact.manifest.v1+json",
   "artifactType": "application/vnd.example.sbom.v1",
   "blobs": [
      {
         "mediaType": "application/gzip",
         "size": 123,
         "digest": "sha256:87923725d74f4bfb94c9e86d64170f7521aad8221a5de834851470ca142da630"
      }
   ],
   "subject": {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 1234,
      "digest": "sha256:cc06a2839488b8bd2a2b99dcdc03d5cfd818eed72ad08ef3cc197aac64c0d0a0"
   },
   "annotations": {
      "org.example.sbom.format": "json",
      "org.opencontainers.artifact.created": "2022-01-01T14:42:55Z"
   }
}`)

func makeTestArtifactManifest() ArtifactManifest {
	return ArtifactManifest{
		MediaType:    v1.MediaTypeArtifactManifest,
		ArtifactType: "application/vnd.example.sbom.v1",
		Blobs: []distribution.Descriptor{
			{
				MediaType: "application/gzip",
				Size:      123,
				Digest:    "sha256:87923725d74f4bfb94c9e86d64170f7521aad8221a5de834851470ca142da630",
			},
		},
		Subject: distribution.Descriptor{
			MediaType: v1.MediaTypeImageManifest,
			Size:      1234,
			Digest:    "sha256:cc06a2839488b8bd2a2b99dcdc03d5cfd818eed72ad08ef3cc197aac64c0d0a0",
		},
		Annotations: map[string]string{
			"org.opencontainers.artifact.created": "2022-01-01T14:42:55Z",
			"org.example.sbom.format":             "json"},
	}
}
func TestArtifactManifest(t *testing.T) {
	testManifest := makeTestArtifactManifest()

	// Test ArtifactManifestFromStruct()
	deserialized, err := ArtifactManifestFromStruct(testManifest)
	if err != nil {
		t.Fatalf("error creating DeserializedArtifactManifest: %v", err)
	}

	// Test DeserializedArtifactManifest.Payload()
	mediaType, canonical, _ := deserialized.Payload()
	if mediaType != v1.MediaTypeArtifactManifest {
		t.Fatalf("unexpected media type: %s", mediaType)
	}

	// Validate DeserializedArtifactManifest.canonical
	p, err := json.MarshalIndent(&testManifest, "", "   ")
	if err != nil {
		t.Fatalf("error marshaling manifest: %v", err)
	}
	if !bytes.Equal(p, canonical) {
		t.Fatalf("manifest bytes not equal: %q != %q", string(canonical), string(p))
	}
	// Check that canonical field matches expected value.
	if !bytes.Equal(expectedArtifactManifestSerialization, canonical) {
		t.Fatalf("manifest bytes not equal: %q != %q", string(canonical), string(expectedArtifactManifestSerialization))
	}

	// Validate DeserializedArtifactManifest.ArtifactManifest
	var unmarshalled DeserializedArtifactManifest
	if err := json.Unmarshal(deserialized.canonical, &unmarshalled); err != nil {
		t.Fatalf("error unmarshaling manifest: %v", err)
	}
	if !reflect.DeepEqual(&unmarshalled, deserialized) {
		t.Fatalf("manifests are different after unmarshaling: %v != %v", unmarshalled, *deserialized)
	}

	// Test DeserializedArtifactManifest.References()
	references := deserialized.References()
	if len(references) != 2 {
		t.Fatalf("unexpected number of references: %d", len(references))
	}
}

func TestValidateArtifactManifest(t *testing.T) {
	artifactManifest := ArtifactManifest{
		MediaType:    v1.MediaTypeArtifactManifest,
		ArtifactType: "example/test",
		Blobs:        []distribution.Descriptor{{Size: 7}},
	}
	imageManifest := Manifest{
		Config: distribution.Descriptor{Size: 1},
		Layers: []distribution.Descriptor{{Size: 2}},
	}
	index := manifestlist.ManifestList{
		Manifests: []manifestlist.ManifestDescriptor{
			{Descriptor: distribution.Descriptor{Size: 9}},
		},
	}
	t.Run("valid", func(t *testing.T) {
		b, err := json.Marshal(artifactManifest)
		if err != nil {
			t.Fatal("unexpected error marshaling manifest", err)
		}
		if err := validateArtifactManifest(b); err != nil {
			t.Error("manifest should be valid", err)
		}
	})
	t.Run("invalid_image_manifest", func(t *testing.T) {
		b, err := json.Marshal(imageManifest)
		if err != nil {
			t.Fatal("unexpected error marshaling image manifest", err)
		}
		if err := validateArtifactManifest(b); err == nil {
			t.Error("image manifest should not be valid")
		}
	})
	t.Run("invalid_index", func(t *testing.T) {
		b, err := json.Marshal(index)
		if err != nil {
			t.Fatal("unexpected error marshaling index", err)
		}
		if err := validateArtifactManifest(b); err == nil {
			t.Error("index should not be valid")
		}
	})
}
