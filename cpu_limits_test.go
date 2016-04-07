package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CPU limits", func() {
	It("reports CPU limits correctly", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			Limits: garden.Limits{CPU: garden.CPULimits{LimitInShares: 42}},
		})

		limit, err := container.CurrentCPULimits()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(limit.LimitInShares).To(Equal(uint64(42)))
	})
})
