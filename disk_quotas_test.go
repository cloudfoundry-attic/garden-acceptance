package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("disk quotas", func() {
	const aliceBytesAlreadyUsed = uint64(12610728) // This may break if the image changes. Would use metrics to get this value, but it's not immediate

	verifyQuotasAcrossUsers := func(rootfs string) {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		byteLimit := aliceBytesAlreadyUsed + (1024 * 1024 * 2)
		err := container.LimitDisk(garden.DiskLimits{ByteHard: byteLimit})
		Ω(err).ShouldNot(HaveOccurred())

		process, err := container.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/zero", "of=/home/alice/junk", "bs=1024M", "count=1"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer := gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "bob", Path: "dd", Args: []string{"if=/dev/zero", "of=/home/bob/junk", "bs=1024M", "count=2"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("dd: can't open '/home/bob/junk': Disk quota exceeded"))
	}

	verifyQuotasOnlyAffectASingleContainer := func(rootfs string) {
		containerWithQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		byteLimit := aliceBytesAlreadyUsed + (1024 * 1024)
		err := containerWithQuota.LimitDisk(garden.DiskLimits{ByteHard: byteLimit})
		Ω(err).ShouldNot(HaveOccurred())

		containerWithoutQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		process, err := containerWithoutQuota.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/zero", "of=/home/alice/junk", "bs=1024M", "count=2"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
	}

	Context("when the container is created from a docker image (#92647640)", func() {
		rootfs := "docker:///cloudfoundry/garden-pm#alice"

		It("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		It("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})
	})

	Context("when the container is created from a directory rootfs (#95436952)", func() {
		rootfs := "/home/vagrant/garden/rootfs/alice"

		It("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		It("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})
	})
})
