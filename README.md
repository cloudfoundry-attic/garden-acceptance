# Garden Acceptance Suite

## Installing Go

    brew install go --with-cc-enable
    export GOPATH=~/go

## Installing the test suite

    go get -t -v -u github.com/cloudfoundry-incubator/garden-acceptance
    go install github.com/onsi/ginkgo/ginkgo

## Usage

### Garden Linux Release

To run these tests, you'll need to clone the [cloudfoundry-incubator/garden-linux-release](https://github.com/cloudfoundry-incubator/garden-linux-release) git repository and then follow the instructions for [running the release in vagrant, locally](https://github.com/cloudfoundry-incubator/garden-linux-release/blob/master/docs/vagrant-bosh.md).  Before running `vagrant up` or `vagrant provision`, you'll want to set the `GARDEN_MANIFEST` environment variable to point to the `vagrant_bosh_deploy_manifest.yml` bosh manifest in this directory:

    export GARDEN_MANIFEST=/path/to/garden-acceptance/vagrant_bosh_deploy_manifest.yml
    vagrant provision

### Generating the rootfs images

The tests rely on a couple of rootfs images being in the `rootfs_images` directory.  These aren't in git, since they're huge.  Here's how to install them:

1. Make sure you can run docker locally (hint: you may need boot2docker).
1. Make the tarball's:
    ```
    git clone git://github.com/cloudfoundry-incubator/diego-dockerfiles.git
    cd diego-dockerfiles/warden-ci
    make fuse-test-rootfs.tar ubuntu-test-rootfs.tar
    ```
1. Compress them into the `rootfs_images` directory:
	  ```
	  gzip -c fuse-test-rootfs.tar > ~/pivotal/go/garden-acceptance/rootfs_images/fusefs.tgz
	  gzip -c ubuntu-test-rootfs.tar > ~/pivotal/go/garden-acceptance/rootfs_images/nestable.tgz
	  ```

### Running Tests

First, set the environment variable `GARDEN_LINUX_RELEASE_DIR` to the directory containing the `garden-linux-release` repository. For example:

    export GARDEN_LINUX_RELEASE_DIR=~/path/to/garden-linux-release

The tests use this to access the vagrant VM for filesystem commands.  With Garden running in vagrant, exposed on `127.0.0.1:7777`, issue the following:

    cd $GOPATH/cloudfoundry-incubator/garden-acceptance
    go get -u github.com/cloudfoundry-incubator/garden
    ginkgo -succinct=true -slowSpecThreshold=150

