package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("disk quotas", func() {
	const aliceBytesAlreadyUsed = uint64(9273344) // This may break if the image changes. Would use metrics to get this value, but it's not immediate

	It("sets a single quota for the whole container", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/garden-pm#alice"})
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
	})

	It("restricts quotas to a single container", func() {
		containerWithQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/garden-pm#alice"})
		byteLimit := aliceBytesAlreadyUsed + (1024 * 1024)
		err := containerWithQuota.LimitDisk(garden.DiskLimits{ByteHard: byteLimit})
		Ω(err).ShouldNot(HaveOccurred())

		containerWithoutQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/garden-pm#alice"})
		process, err := containerWithoutQuota.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/zero", "of=/home/alice/junk", "bs=1024M", "count=2"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
	})
})
