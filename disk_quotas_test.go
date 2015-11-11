package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("disk quotas", func() {
	verifyQuotasAcrossUsers := func(rootfs string) {
		rootfsBytesUsed := rootFSDiskUsage(rootfs)
		var byteLimit uint64 = rootfsBytesUsed + (1024 * 1024 * 2)
		container := createContainer(gardenClient, garden.ContainerSpec{
			RootFSPath: rootfs,
			Limits: garden.Limits{
				Disk: garden.DiskLimits{ByteHard: byteLimit, Scope: garden.DiskLimitScopeTotal},
			},
		})

		process, err := container.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/alice/junk", "bs=1M", "count=1"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer := gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "bob", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/bob/junk", "bs=1M", "count=10"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("dd: writing '/home/bob/junk': No space left on device"))
	}

	verifyQuotasOnlyAffectASingleContainer := func(rootfs string) {
		rootfsBytesUsed := rootFSDiskUsage(rootfs)
		var byteLimit uint64 = rootfsBytesUsed + (1024 * 1024)
		createContainer(gardenClient, garden.ContainerSpec{
			RootFSPath: rootfs,
			Limits: garden.Limits{
				Disk: garden.DiskLimits{ByteHard: byteLimit, Scope: garden.DiskLimitScopeTotal},
			},
		})

		containerWithoutQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		process, err := containerWithoutQuota.Run(
			garden.ProcessSpec{User: "root", Path: "dd", Args: []string{"if=/dev/zero", "of=/etc/junk", "bs=1M", "count=4"}},
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

func totalContainerDiskUsage(container garden.Container) uint64 {
	metrics, err := gardenClient.BulkMetrics([]string{container.Handle()})
	Ω(err).ShouldNot(HaveOccurred())
	return metrics[container.Handle()].Metrics.DiskStat.TotalBytesUsed
}

func rootFSDiskUsage(rootFSPath string) uint64 {
	container := createContainer(gardenClient, garden.ContainerSpec{
		RootFSPath: rootFSPath,
		Limits: garden.Limits{
			Disk: garden.DiskLimits{ByteHard: 1024 * 1204 * 1024},
		},
	})
	metrics, err := container.Metrics()
	Ω(err).ShouldNot(HaveOccurred())
	return metrics.DiskStat.TotalBytesUsed
}
