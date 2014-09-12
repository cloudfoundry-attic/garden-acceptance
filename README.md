# Garden Acceptance Suite

To use (assuming you have Garden running locally on `127.0.0.1:7777`)

```
go get -t -v -u github.com/cloudfoundry-incubator/garden-acceptance
go install github.com/onsi/ginkgo/ginkgo
cd $GOPATH/cloudfoundry-incubator/garden-acceptance
ginkgo
```