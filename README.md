# Garden Acceptance Suite

## Installing Go

```
brew install go
export GOPATH=~/go
```

## Usage

### Garden Linux Release

To run these tests, you'll need to clone the [garden-linux-release](https://github.com/cloudfoundry-incubator/garden-linux-release) git repository and then follow the instructions for [running the release in vagrant, locally](https://github.com/cloudfoundry-incubator/garden-linux-release/blob/master/docs/vagrant-bosh.md).

### Running Tests

First, set the environment variable `GARDEN_LINUX_RELEASE_DIR` to the directory containing the garden-linux-release repository. For example:
```
export GARDEN_LINUX_RELEASE_DIR=/home/xxx/workspace/garden-linux-release
```

The tests use this to access the vagrant vm.

With Garden running in vagrant, exposed on `127.0.0.1:7777`, issue the following:
```
go get -t -v -u github.com/cloudfoundry-incubator/garden-acceptance
go install github.com/onsi/ginkgo/ginkgo
cd $GOPATH/cloudfoundry-incubator/garden-acceptance
ginkgo
```

