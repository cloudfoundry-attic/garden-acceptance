package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("properties", func() {
	var container garden.Container

	Context("with initial properties", func() {
		BeforeEach(func() {
			container = createContainer(gardenClient, garden.ContainerSpec{
				Properties: garden.Properties{"foo": "bar"},
			})
		})

		It("can CRUD properties", func() {
			value, err := container.Property("foo")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(value).Should(Equal("bar"))

			err = container.SetProperty("foo", "baz")
			Ω(err).ShouldNot(HaveOccurred())
			value, err = container.Property("foo")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(value).Should(Equal("baz"))

			err = container.SetProperty("fiz", "buz")
			Ω(err).ShouldNot(HaveOccurred())

			err = container.RemoveProperty("foo")
			Ω(err).ShouldNot(HaveOccurred())

			_, err = container.Property("foo")
			Ω(err.Error()).Should(ContainSubstring("property does not exist: foo"))

			_, err = container.Property("bar")
			Ω(err.Error()).Should(ContainSubstring("property does not exist: bar"))

			properties, err := container.Properties()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(properties).Should(ContainElement("buz"))
		})

		It("can filter containers by property", func() {
			createContainer(gardenClient, garden.ContainerSpec{
				Properties: garden.Properties{"foo": "othercontainer"},
			})
			createContainer(gardenClient, garden.ContainerSpec{})

			containers, err := gardenClient.Containers(map[string]string{"foo": "bar"})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(containers).Should(ConsistOf(container))
		})
	})

	Context("without initial properties", func() {
		BeforeEach(func() {
			container = createContainer(gardenClient, garden.ContainerSpec{})
		})

		It("can set a property (#87599106)", func() {
			err := container.SetProperty("foo", "bar")
			Ω(err).ShouldNot(HaveOccurred())

			value, err := container.Property("foo")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(value).Should(Equal("bar"))
		})
	})
})
