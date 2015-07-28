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
	const aliceBytesAlreadyUsed = uint64(12610728) // This may break if the image changes. Would use metrics to get this value, but it's not immediate

	PIt("mounts /sys read-only", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "mount",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).To(Equal(0))
		process, err = container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ls",
			Args: []string{"-l", "/"},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).To(Equal(0))

		fmt.Println(string(buffer.Contents()))
	})

	PIt("something to do with disk usage reporting", func() {
		rootfs := "docker:///cloudfoundry/garden-pm#alice"
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		time.Sleep(time.Second * 60)
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

		time.Sleep(time.Second * 60)
		metricsAfterWritingData, err := container.Metrics()
		Ω(err).ShouldNot(HaveOccurred())
		fmt.Println("")
		fmt.Println(metricsAfterWritingData.DiskStat.TotalBytesUsed)
		fmt.Println(metricsAfterWritingData.DiskStat.ExclusiveBytesUsed)
	})

	verifyQuotasAcrossUsers := func(rootfs string) {
		oneMb := 1024 * 1024
		totalUsed := 2727936
		// exclusiveUsed := 196608
		byteLimit := totalUsed + (oneMb * 2)
		container := createContainer(
			gardenClient,
			garden.ContainerSpec{
				RootFSPath: rootfs,
				Limits: garden.Limits{
					Disk: garden.DiskLimits{ByteHard: uint64(byteLimit), Scope: garden.DiskLimitScopeTotal},
				}})

		process, err := container.Run(
			garden.ProcessSpec{User: "alice", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/alice/junk", "bs=1M", "count=1"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer := gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "bob", Path: "dd", Args: []string{"if=/dev/urandom", "of=/home/bob/junk", "bs=1024M", "count=1"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("dd: can't open '/home/bob/junk': Disk quota exceeded"))
	}

	verifyQuotasOnlyAffectASingleContainer := func(rootfs string) {
		byteLimit := aliceBytesAlreadyUsed + (1024 * 1024)
		createContainer(
			gardenClient,
			garden.ContainerSpec{
				RootFSPath: rootfs,
				Limits: garden.Limits{
					Disk: garden.DiskLimits{ByteHard: byteLimit},
				}})

		containerWithoutQuota := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: rootfs})
		process, err := containerWithoutQuota.Run(
			garden.ProcessSpec{User: "root", Path: "dd", Args: []string{"if=/dev/zero", "of=/junk", "bs=1024M", "count=2"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
	}

	PIt("limits...", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			Limits: garden.Limits{
				Bandwidth: garden.BandwidthLimits{RateInBytesPerSecond: 420, BurstRateInBytesPerSecond: 421},
				CPU:       garden.CPULimits{LimitInShares: 42},
				// Memory:    garden.MemoryLimits{LimitInBytes: 42000},
			},
		})

		err := container.LimitMemory(garden.MemoryLimits{LimitInBytes: 42000})
		Ω(err).ShouldNot(HaveOccurred())

		bandwidthLimits, err := container.CurrentBandwidthLimits()
		Ω(err).ShouldNot(HaveOccurred())
		CPULimits, err := container.CurrentCPULimits()
		Ω(err).ShouldNot(HaveOccurred())
		memoryLimits, err := container.CurrentMemoryLimits()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(bandwidthLimits.RateInBytesPerSecond).Should(BeNumerically("==", 420))
		Ω(bandwidthLimits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 421))
		Ω(CPULimits.LimitInShares).Should(BeNumerically("==", 42))
		Ω(memoryLimits.LimitInBytes).Should(BeNumerically("==", 42000))
	})

	Context("when the container is created from a docker image (#92647640)", func() {
		rootfs := "docker:///cloudfoundry/garden-pm#alice"

		FIt("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		It("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})
	})

	Context("when the container is created from a directory rootfs (#95436952)", func() {
		rootfs := "/var/vcap/packages/rootfs/alice"

		// rootfs permissions again
		PIt("sets a single quota for the whole container", func() {
			verifyQuotasAcrossUsers(rootfs)
		})

		// fails every second time... wtf?
		PIt("restricts quotas to a single container", func() {
			verifyQuotasOnlyAffectASingleContainer(rootfs)
		})
	})
})
