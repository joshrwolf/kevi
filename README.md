# kevi

`kevi` is a kubernetes appliance builder/deployer (sys*keví* = Greek for "appliance", or so says google translate).

`kevi` is for those who primarily develop on internet connected systems, but deploy exclusively to disconnected environments.
It makes it simple to collect kubernetes dependencies and deploy them, making the airgap deployment process transparent.

`kevi` runs as an operator in your cluster, and synchronizes the resources you define.  It uses a simple configuration format to define resources that doubles as it's CRD api spec, can auto-deploy itself to any CNCF compatible cluster, and has zero dependencies.

[![asciicast](https://asciinema.org/a/sm5LLweUzOQjuRjQ4IGZYr6Gv.svg)](https://asciinema.org/a/sm5LLweUzOQjuRjQ4IGZYr6Gv)

## Quickstart

### Prerequisites

`kevi` makes _no_ assumptions about your cluster, if it's a CNCF certified distribution, things will work.
This also means `kevi` doesn't help you install a cluster, but it's 2022, chances are you don't need help with this anymore.
If you're looking for some inspiration, try [k3s](https://k3s.io) it's a <50MB static binary, and  _stupid_ simple to deploy almost anywhere, including airgapped.

For storage, `kevi` assumes you have an accessible OCI registry with push permissions.  Again, it's 2022, blah blah blah.
If you need inspiration for a self-hosted option, [distribution](https://github.com/distribution/distribution) is battle tested and production capable, all in a ~20MB binary with zero dependencies, and supports multiple storage backends.

### Example

For convenience, `kevis` configuration is also it's CRD.  For the example below, we'll use the following configuration/CRD:

```yaml
apiVersion: packages.cattle.io/v1alpha1
kind: Kevi
metadata:
  name: demo
  namespace: default
spec:
  # Packages definition is designed to be flexible, catering to however you define manifests
  packages:
      # Path to local raw manifests
    - name: raw-manifests
      manifest:
        path: testdata/raw-manifests/

      # Path to local manifests constructed via Kustomize
    - name: kustomize-manifests
      manifest:
        path: testdata/kustomize
      # Specify extra list of images that may not be present in manifests but still vital to the deployment
      images:
        - alpine:latest

      # Path to local chart archive (or directory with valid Chart.yaml)
    - name: podinfo
      chart:
        path: testdata/podinfo-6.0.3.tgz

      # Remote chart
    - name: loki-chart
      chart:
        name: loki
        repoUrl: https://grafana.github.io/helm-charts
```

Notice how some of the paths above are relative? We're going to assume you're running the following commands (not the airgap ones) from this repositories root.

#### Online

The following command(s) assume you're running from the root of this repository (you cloned this right?)

```bash
# create a portable package from one or more kevis
kevi pack -f testdata/demo-kevi.yaml --archive
```

Once complete, you'll notice a `packages.tar.gz` file got created in your current working directory.  Toss this over the fence!

#### Offline

The following command(s) assume you're running on a node with kubectl access to your target cluster.

```bash
# Deploy the package you just created and all it's dependent resources to your airgapped cluster
kevi deploy $registry -f packages.tar.gz
```

Sit back and watch `kevi` get to work!
 and more
### What just happened?

Apart from (hopefully) successfully deploying all your packages to the cluster, `kevi` did that in a few steps:

- Installed itself as an operator into your cluster ([details](#q-where-does-the-kevi-image-deployed-in-my-cluster-come-from))
- Copied all of your package dependencies into your registry ([details](#q-why-do-you-need-a-registry))
- Deployed your packages without needing any airgap specific modifications ([details-1](#q-how-are-images-sourced-from-my-registry-without-modifying-the-manifests), [details-2](#q-how-are-my-manifests-deployed))

If you're not a fan of that much transparency, all those steps above and more can be run individually (`kevi --help`).

## Q/A

##### Q: It's 2022 but I still want help automagically deploying a cluster and oci registry!

> Hang tight! A push button solution is coming soon.

##### Q: Why do you need a registry?

> Apart from storing packages images for k8s to pull from, the package's manifest content (raw, chart, etc...) are also stored and fetched from the registry!  This helps easily distribute immutable content beyond just a single hosts filesystem, and means `kevi` works across the entire cluster.

##### Q: Where does the `kevi` image deployed in my cluster come from?

> `kevi` creates an OCI image of itself entirely from code, pushes the result to the specified registry, and instructs Kubernetes to pull from the produced image.
This image is based on `gcr.io/distroless/static:nonroot`, _almost_ identical to the final image in `./Dockerfile` (modtimes, metadata, and layer history will differ since it's built "on demand"), and requires nothing (no docker, buildkit, buildah, etc...).
The downside is the image produced _must_ be the same architecture and OS of the executable that ran it.
The upside is you don't need to lug around a separate image just for the controller, which is ultimately one less thing to worry about when airgapping.

##### Q: How are images sourced from my registry without modifying the manifests?

> `kevi` is a controller, but also a webhook! Specifically a Mutating webhook.  When `kevi` is installed (by itself), it configures a Mutating webhook that tells Kubernetes to send all `pod` CREATE/UPDATE requests to `kevi` for relocation.
Since you install `kevi` by specifying a registry, that registry becomes the relocation target.
This means `kevi` doesn't have to know where every image is defined in the manifests structure.  In the age of operators, images could be defined anywhere, sometimes not in standard locations in manifests, or even at all!
`kevi` don't care, it's not the one making the pod requests, it's just mutating requests that kubernetes is making.  This guarantees _all_ images created in the cluster can be sourced from the content source registry of choice.

##### Q: How are packages stored?

> [OCI layouts](https://github.com/opencontainers/image-spec/blob/main/image-layout.md)! Following the theme of "everything can be distrubted via an OCI compatible registry", we have the "everything can be _collected_ via an OCI format".
When you `pack` packages, the manifests, charts, and images are all stored locally in an OCI layout.
Since OCI layouts are an ever increasingly common standard, several tools exist that let us do some cool things with these layouts (in the future, stay tuned!).

##### Q: How are my manifests deployed?

> The Kubernetes community is [great](https://twitter.com/vicnastea/status/1469822437416071170?t=rVwEp4BMrEyJA-aybMK_sQ&s=19) at consolidating on a way to define manifests.  Ask 10 people and you'll get 20 different answers.
`kevi` assumes you're using some form of raw manifests, kustomize, _or_ helm to deploy your manifests.  We think this covers the vast majority of users, but we know we missed some people with this assumption.
The heavy lifting and actual synchronization comes courtesy of the gitops-engine.  All package sources are eventually turned into deployable objects consumable by the gitops-engine.
Raw manifests _and_ kustomizations are all deployed as a kustomization.  In the case of raw manifests, a `kustomization.yaml` will be generated (shoutout to the flux authors for this idea!).
Finally, charts are templated out to their resources.

##### Q: I don't have root access :(

> That's fine! `kevi` runs entirely rootless.  However, it _does_ need `verb=*` access to any kubernetes resource you expect it to manage... so we can't really pat our back on this one.

## What doesn't work yet?

`kevi` is super transparent, and so are we! Most of the "hard" things are functional, but there are a lot of rough edges.

Here's a list of things we know currently suck but are working on:

Current areas that need TLC are:

Problem: logging, it sucks!

> Solution: log all the things!

Problem: Authentication to insecure registries

> Solution: allow user to better customize registry authentication (or tls checks in general)

Problem: Chart values are not customizable

> Solution: Add `values` to the `chart` spec that are passed in at chart render time

Problem: Last mile manifest configuration doesn't exist

> Solution: Allow each package to be configured with strategic merge patches (directly for raw/kustomize, and through values for charts)

Problem: `kevi` installation is not configurable

> Solution: Make `kevi` installation configurable... duh

Problem: Overly permissive cluster rbac

> Solution: Not sure... since we need `verb=*` for whatever resource you tell `kevi` to manage, it's tough to get away from this.  Maybe another controller that dynamically configures the RBAC based on the packages defined? Or even just dynamically configuring the RBAC based on the packages.  Or maybe you just trust us? Yeah?

## Acknowledgements

We didn't make this ourselves.  `kevi` wouldn't be possible without the excellent open source community, in particular:

- gitops-engine: for doing most of the heavy lifting
- flux: for excellent libraries
- oras: for OCI'ing all the things
- go-containerregistry: for making working with images easy
- controller-runtime: for making operators easy

