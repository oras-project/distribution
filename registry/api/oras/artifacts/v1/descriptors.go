package v1

import (
	"net/http"

	"github.com/distribution/distribution/v3/reference"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
)

var (
	nameParameterDescriptor = v2.ParameterDescriptor{
		Name:        "name",
		Type:        "string",
		Format:      reference.NameRegexp.String(),
		Required:    true,
		Description: `Name of the target repository.`,
	}

	digestParameterDescriptor = v2.ParameterDescriptor{
		Name:        "digest",
		Type:        "string",
		Format:      reference.DigestRegexp.String(),
		Required:    true,
		Description: `Digest of the target manifest.`,
	}

	hostHeader = v2.ParameterDescriptor{
		Name:        "Host",
		Type:        "string",
		Description: "Standard HTTP Host Header. Should be set to the registry host.",
		Format:      "<registry host>",
		Examples:    []string{"registry-1.docker.io"},
	}

	authHeader = v2.ParameterDescriptor{
		Name:        "Authorization",
		Type:        "string",
		Description: "An RFC7235 compliant authorization header.",
		Format:      "<scheme> <token>",
		Examples:    []string{"Bearer dGhpcyBpcyBhIGZha2UgYmVhcmVyIHRva2VuIQ=="},
	}
)

var routeDescriptors = []v2.RouteDescriptor{

	{
		Name:        RouteNameReferrers,
		Path:        "/oras/artifacts/v1/{name:" + reference.NameRegexp.String() + "}/manifests/{digest:" + reference.DigestRegexp.String() + "}/referrers",
		Entity:      "Referrers",
		Description: "Retrieve information about artifacts that reference this manifest.",
		Methods: []v2.MethodDescriptor{
			{
				Method:      "GET",
				Description: "Fetch a list of referrers.",
				Requests: []v2.RequestDescriptor{
					{
						Name:        "Referrers",
						Description: "Return a list of all referrers of the given manifest filtered by the given artifact type.",
						Headers: []v2.ParameterDescriptor{
							hostHeader,
							authHeader,
						},
						PathParameters: []v2.ParameterDescriptor{
							nameParameterDescriptor,
							digestParameterDescriptor,
						},
						QueryParameters: []v2.ParameterDescriptor{
							{
								Name:        "artifactType",
								Type:        "query",
								Format:      "<string>",
								Description: `Artifact type of the requested referrers.`,
							},
							{
								Name:        "n",
								Type:        "query",
								Format:      "<integer>",
								Description: `Pagination parameter indicating the requested number of results`,
							},
						},
						Successes: []v2.ResponseDescriptor{
							{
								StatusCode:  http.StatusOK,
								Description: "A list of referrers of the given manifest filtered by the given artifact type.",
								Headers: []v2.ParameterDescriptor{
									{
										Name:        "Content-Length",
										Type:        "integer",
										Description: "Length of the JSON response body.",
										Format:      "<length>",
									},
									{
										Name:        "Content-Type",
										Type:        "string",
										Description: "Type of content in the response body.",
										Format:      "<string>",
									},
									{
										Name:        "Link",
										Type:        "string",
										Description: "RFC5988 Link header with next relationship. Used for pagination.",
										Format:      "<string>",
									},
								},
								Body: v2.BodyDescriptor{
									ContentType: "application/json",
									Format: `
{
	"references": [
			{
				"digest": "<string>",
				"mediaType": "<string>",
				"artifactType": "<string>",
				"size": <integer>
			},
			...
	]
}
									`,
								},
							},
						},
					},
				},
			},
		},
	},
}
