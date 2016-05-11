# Garden Acceptance Suite

## Running the tests

1. Set up [BOSH Lite](https://github.com/cloudfoundry/bosh-lite).
1. Clone
   [Garden-RunC-Release](https://github.com/cloudfoundry-incubator/garden-runc-release)
   in a neighboring directory to this repo. Update submodules and create/upload
   a release.
1. In the `release` subdirectory of this repo, create and upload a release.
1. Deploy using the `manifests/bosh-lite.yml` manifest.
1. `ginkgo`!

## Updating Docker images

1. `docker login` and authenticate using the `cloudfoundry` account
1. `cd docker`
1. `./build_and_push`
