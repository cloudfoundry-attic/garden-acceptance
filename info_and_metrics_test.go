package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("info and metrics", func() {
	Describe("Container.Info()", func() {
		It("returns a container IP", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{Network: "10.1.1.1/16"})
			info, err := container.Info()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(info.ContainerIP).Should(Equal("10.1.1.1"))
		})
	})

	Describe("Client.BulkInfo()", func() {
		It("returns IPs for multiple containers", func() {
			container1 := createContainer(gardenClient, garden.ContainerSpec{Network: "10.1.1.1/16"})
			container2 := createContainer(gardenClient, garden.ContainerSpec{Network: "10.1.1.2/16"})
			handle1 := container1.Handle()
			handle2 := container2.Handle()

			infos, err := gardenClient.BulkInfo([]string{handle1, handle2})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(infos[handle1].Info.ContainerIP).Should(Equal("10.1.1.1"))
			Ω(infos[handle2].Info.ContainerIP).Should(Equal("10.1.1.2"))
		})
	})

	Describe("Container.Metrics()", func() {
		It("returns the CPU Usage", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})
			metrics, err := container.Metrics()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(metrics.CPUStat.Usage).Should(BeNumerically(">", 0))
		})

		It("returns network statistics", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})
			preRequestMetrics, err := container.Metrics()
			Ω(err).ShouldNot(HaveOccurred())

			process, err := container.Run(garden.ProcessSpec{User: "root", Path: "wget", Args: []string{"http://example.com"}}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))

			postRequestMetrics, err := container.Metrics()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(postRequestMetrics.NetworkStat.TxBytes).Should(BeNumerically(">", preRequestMetrics.NetworkStat.TxBytes))
			Ω(postRequestMetrics.NetworkStat.RxBytes).Should(BeNumerically(">", preRequestMetrics.NetworkStat.RxBytes))
		})

		It("returns disk usage info", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{
				Limits: garden.Limits{
					Disk: garden.DiskLimits{
						Scope:    garden.DiskLimitScopeExclusive,
						ByteHard: 1024 * 1024 * 100,
					},
				},
			})

			preDdMetrics, err := container.Metrics()
			Ω(err).ShouldNot(HaveOccurred())

			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "dd",
				Args: []string{"if=/dev/urandom", "of=/etc/junk", "bs=1M", "count=1"},
			}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))

			postDdMetrics, err := container.Metrics()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(postDdMetrics.DiskStat.ExclusiveBytesUsed).Should(
				BeNumerically(">", preDdMetrics.DiskStat.ExclusiveBytesUsed))
		})
	})

	Describe("Client.BulkMetrics()", func() {
		It("returns the CPU Usage (#90241386)", func() {
			createContainer(gardenClient, garden.ContainerSpec{Handle: "foo"})
			metricsEntries, err := gardenClient.BulkMetrics([]string{"foo"})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(metricsEntries["foo"].Metrics.CPUStat.Usage).Should(BeNumerically(">", 0))
		})

		It("returns disk usage info", func() {
			handle := "myFirstContainer"
			container := createContainer(gardenClient, garden.ContainerSpec{
				Handle: handle,
				Limits: garden.Limits{
					Disk: garden.DiskLimits{
						Scope:    garden.DiskLimitScopeExclusive,
						ByteHard: 1024 * 1024 * 100,
					},
				},
			})

			bulk, err := gardenClient.BulkMetrics([]string{handle})
			Ω(err).ShouldNot(HaveOccurred())
			preDdMetrics := bulk[handle]

			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "dd",
				Args: []string{"if=/dev/urandom", "of=/etc/junk", "bs=1M", "count=1"},
			}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))

			bulk, err = gardenClient.BulkMetrics([]string{handle})
			Ω(err).ShouldNot(HaveOccurred())
			postDdMetrics := bulk[handle]

			Ω(postDdMetrics.Metrics.DiskStat.ExclusiveBytesUsed).Should(
				BeNumerically(">", preDdMetrics.Metrics.DiskStat.ExclusiveBytesUsed))
		})
	})
})
