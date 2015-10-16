package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("disk quotas", func() {
	verifyQuotasAcrossUsers := func(rootfs string) {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		metrics, err := container.Metrics()
		Ω(err).ShouldNot(HaveOccurred())
		var byteLimit uint64 = metrics.DiskStat.TotalBytesUsed + (2 * 1024 * 1024)
		err = container.LimitDisk(garden.DiskLimits{ByteHard: byteLimit, Scope: garden.DiskLimitScopeTotal})
		Ω(err).ShouldNot(HaveOccurred())

		process, err := container.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/alice/junk", "bs=1M", "count=1"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer := gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "bob", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/bob/junk", "bs=1M", "count=2"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("dd: writing '/home/bob/junk': Disk quota exceeded"))
	}

	verifyQuotasOnlyAffectASingleContainer := func(rootfs string) {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		metrics, err := container.Metrics()
		Ω(err).ShouldNot(HaveOccurred())
		var byteLimit uint64 = metrics.DiskStat.TotalBytesUsed + (1024 * 1024)
		err = container.LimitDisk(garden.DiskLimits{ByteHard: byteLimit, Scope: garden.DiskLimitScopeTotal})
		Ω(err).ShouldNot(HaveOccurred())

		containerWithoutQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		process, err := containerWithoutQuota.Run(
			garden.ProcessSpec{User: "root", Path: "dd", Args: []string{"if=/dev/zero", "of=/junk", "bs=1M", "count=2"}},
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
		rootfs := "/var/vcap/packages/rootfs/alice"

		It("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		It("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})
	})
})
