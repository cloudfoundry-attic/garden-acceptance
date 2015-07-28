package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("user mapping", func() {
	validatePermissions := func(rootFSPath string) {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootFSPath})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(
			garden.ProcessSpec{User: "vcap", Path: "touch", Args: []string{"/home/alice/not_me"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("touch: /home/alice/not_me: Permission denied"))

		process, err = container.Run(
			garden.ProcessSpec{User: "alice", Path: "touch", Args: []string{"/home/alice/me"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		process, err = container.Run(
			garden.ProcessSpec{User: "root", Path: "touch", Args: []string{"/i_am_root"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer = gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "alice", Path: "touch", Args: []string{"/i_am_not_root"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("touch: /i_am_not_root: Permission denied"))
	}

	// Needs permissions to be preserved in rootfs
	PIt("maintains permissions from a garden directory rootfs (#92808274)", func() {
		validatePermissions("/var/vcap/packages/rootfs/alice")
	})

	It("maintains permissions from docker images (#91955652)", func() {
		validatePermissions("docker:///cloudfoundry/garden-pm#alice")
	})
})
