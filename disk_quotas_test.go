package garden_acceptance_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("disk quotas", func() {
	PIt("something to do with disk usage reporting", func() {
		rootfs := "docker:///cloudfoundry/garden-pm#alice"
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		metricsBeforeWritingData, err := container.Metrics()
		Ω(err).ShouldNot(HaveOccurred())
		fmt.Println("")
		fmt.Println(metricsBeforeWritingData.DiskStat.TotalBytesUsed)
		fmt.Println(metricsBeforeWritingData.DiskStat.ExclusiveBytesUsed)

		process, err := container.Run(
			garden.ProcessSpec{User: "root", Path: "dd", Args: []string{"if=/dev/random", "of=/home/alice/junk", "bs=1024M", "count=1"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		metricsAfterWritingData, err := container.Metrics()
		Ω(err).ShouldNot(HaveOccurred())
		fmt.Println("")
		fmt.Println(metricsAfterWritingData.DiskStat.TotalBytesUsed)
		fmt.Println(metricsAfterWritingData.DiskStat.ExclusiveBytesUsed)
	})

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

	XIt("limits...", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			Limits: garden.Limits{
				// Bandwidth: garden.BandwidthLimits{RateInBytesPerSecond: 420, BurstRateInBytesPerSecond: 421},
				// CPU:       garden.CPULimits{LimitInShares: 42},
				Memory: garden.MemoryLimits{LimitInBytes: 45056},
			},
		})

		// bandwidthLimits, err := container.CurrentBandwidthLimits()
		// Ω(err).ShouldNot(HaveOccurred())
		// CPULimits, err := container.CurrentCPULimits()
		// Ω(err).ShouldNot(HaveOccurred())
		memoryLimits, err := container.CurrentMemoryLimits()
		Ω(err).ShouldNot(HaveOccurred())

		// Ω(bandwidthLimits.RateInBytesPerSecond).Should(BeNumerically("==", 420))
		// Ω(bandwidthLimits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 421))
		// Ω(CPULimits.LimitInShares).Should(BeNumerically("==", 42))
		Ω(memoryLimits.LimitInBytes).Should(BeNumerically("==", 45056))
	})

	Context("when the container is created from a docker image (#92647640)", func() {
		rootfs := "docker:///cloudfoundry/garden-pm#alice"

		It("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		It("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})

		XIt("does not create the container if it will immediately exceed its disk quota", func() {
			_, err := gardenClient.Create(garden.ContainerSpec{
				RootFSPath: "docker:///cloudfoundry/garden-pm#alice",
				// Limits: garden.Limits{
				// 	Disk: garden.DiskLimits{ByteHard: 512, Scope: garden.DiskLimitScopeTotal},
				// },
			})
			// Ω(err).Should(MatchError(ContainSubstring("quota exceeded")))
			Ω(err).ShouldNot(HaveOccurred())

			time.Sleep(time.Second)

			Ω(gardenClient.Containers(garden.Properties{})).Should(HaveLen(0))
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
