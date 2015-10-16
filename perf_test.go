package garden_acceptance_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = FDescribe("things", func() {
	It("setuid", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: false, RootFSPath: "/var/vcap/packages/rootfs/alice"})
		buffer := gbytes.NewBuffer()
		_, err := container.Run(garden.ProcessSpec{
			User: "alice",
			Path: "ping",
			Args: []string{"8.8.8.8"},
		}, recordedProcessIO(buffer))
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(time.Second * 2)
		fmt.Println(string(buffer.Contents()))
	})
})
