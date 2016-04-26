package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("memory limits", func() {
	It("sets a memory limit", func() {
		limitInBytes := uint64(1024 * 1024 * 10)
		container := createContainer(gardenClient, garden.ContainerSpec{
			Limits: garden.Limits{Memory: garden.MemoryLimits{LimitInBytes: limitInBytes}},
		})

		memoryLimit, err := container.CurrentMemoryLimits()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(memoryLimit.LimitInBytes).To(Equal(limitInBytes))

		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "dd",
			Args: []string{"if=/dev/urandom", "of=/dev/shm/not-too-big", "bs=1M", "count=8"},
		}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		process, err = container.Run(garden.ProcessSpec{
			User: "root",
			Path: "dd",
			Args: []string{"if=/dev/urandom", "of=/dev/shm/too-big", "bs=1M", "count=11"},
		}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
	})
})
