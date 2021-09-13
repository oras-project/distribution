[[__TOC__]]

# ORAS Artifacts Distribution

This document describes an experimental prototype that implements the
[ORAS Artifact Manifest](https://github.com/oras-project/artifacts-spec) spec.

## Implementation

To power the [/referrers](https://github.com/oras-project/artifacts-spec/blob/main/manifest-referrers-api.md) API, the
referrers of a manifest are indexed in the repository store. The following example illustrates the creation of this
index.

The `nginx:v1` image is already persisted:

- repository: `nginx`
- digest: `sha256:111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m`
- tag: `v1.0`

The repository store layout is represented as:

```bash
<root>
└── v2
    └── repositories
        └── nginx
            └── _manifests
                └── revisions
                    └── sha256
                        └── 111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m
                            └── link
```

Push a signature as blob and an ORAS Artifact that contains a blobs property referencing the signature, with the
following properties:

- digest: `sha256:222ibbf80b44ce6be8234e6ff90a1ac34acbeb826903b02cfa0da11c82cb222i`
- `subjectManifest` digest: `sha256:111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m`
- `artifactType`: `application/vnd.example.artifact`

On `PUT`, the artifact appears as a manifest revision. Additionally, an index entry is created under
the subject to facilitate a lookup to the referrer. The index path where the entry is added is
`/ref/<artifactType>`, as shown below.

```
<root>
└── v2
    └── repositories
        └── nginx
            └── _manifests
                └── revisions
                    └── sha256
                        ├── 111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m
                        │   ├── link
                        │   └── ref
                        │       └── digest(application/vnd.example.artifact)
                        │           └── sha256
                        │               └── 222ibbf80b44ce6be8234e6ff90a1ac34acbeb826903b02cfa0da11c82cb222i
                        │                   └── link
                        └── 222ibbf80b44ce6be8234e6ff90a1ac34acbeb826903b02cfa0da11c82cb222i
                            └── link
```

Push another ORAS artifact with the following properties:

- digest: `sha256:333ic0c33ebc4a74a0a554c86ac2b28ddf3454a5ad9cf90ea8cea9f9e75c333i`
- `subjectManifest` digest: `sha256:111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m`
- `artifactType`: `application/vnd.another.example.artifact`

This results in an addition to the index as shown below.

```
<root>
└── v2
    └── repositories
        └── nginx
            └── _manifests
                └── revisions
                    └── sha256
                        ├── 111ma2d22ae5ef400769fa51c84717264cd1520ac8d93dc071374c1be49a111m
                        │   ├── link
                        │   └── ref
                        │       ├── digest(application/vnd.example.artifact)
                        │       │   └── sha256
                        │       │       └── 222ibbf80b44ce6be8234e6ff90a1ac34acbeb826903b02cfa0da11c82cb222i
                        │       │           └── link
                        │       └── digest(application/vnd.another.example.artifact)
                        │           └── sha256
                        │               └── 333ic0c33ebc4a74a0a554c86ac2b28ddf3454a5ad9cf90ea8cea9f9e75c333i
                        │                   └── link
                        ├── 222ibbf80b44ce6be8234e6ff90a1ac34acbeb826903b02cfa0da11c82cb222i
                        │   └── link
                        └── 333ic0c33ebc4a74a0a554c86ac2b28ddf3454a5ad9cf90ea8cea9f9e75c333i
                            └── link
```
