package ocischema

// unknownDocument represents a manifest, manifest list, or index that has not
// yet been validated. This type is used for validating byte content.
type unknownDocument struct {
	MediaType string      `json:"mediaType"`           // used to recognize ArtifactManifest
	Config    interface{} `json:"config"`              // used to recognize ImageManifest
	Manifests interface{} `json:"manifests,omitempty"` // used to recognize index
}
