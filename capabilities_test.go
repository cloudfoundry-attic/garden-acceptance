package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

// TODO: Add capcheck to garden-acceptance bosh release, bind mount in to containers
var _ = PDescribe("dropping capabilities", func() {
	var container garden.Container

	Context("for privileged containers", func() {
		BeforeEach(func() {
			container = createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
		})

		It("doesn't drop any when the process is run as root", func() {
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "/bin/capcheck",
			}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0)) // capcheck exits 0 if all caps are available
		})

		It("drops 'the list' - CAP_SYS_ADMIN when the process is run as non-root", func() {
			buffer := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{
				User: "alice",
				Path: "/bin/capcheck",
			}, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			process.Wait()

			Ω(buffer).Should(gbytes.Say("CAP_SYS_ADMIN: Create bind mount succeeded"))
			Ω(buffer).Should(gbytes.Say("CAP_MKNOD: Failed to make a node"))
			Ω(buffer).Should(gbytes.Say("CAP_NET_BIND_SERVICE: Create listener succeeded"))
		})
	})

	Context("for unprivileged containers", func() {
		BeforeEach(func() {
			container = createContainer(gardenClient, garden.ContainerSpec{})
		})

		It("drops 'the list' when the process is run as root", func() {
			buffer := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "/bin/capcheck",
			}, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			process.Wait()

			Ω(buffer).Should(gbytes.Say("CAP_SYS_ADMIN: Failed to create a bind mount"))
			Ω(buffer).Should(gbytes.Say("CAP_MKNOD: Failed to make a node"))
			Ω(buffer).Should(gbytes.Say("CAP_NET_BIND_SERVICE: Create listener succeeded"))
		})

		It("drops 'the list' when the process is run as non-root", func() {
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "chmod",
				Args: []string{"u+s", "/bin/capcheck"},
			}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))

			buffer := gbytes.NewBuffer()
			process, err = container.Run(garden.ProcessSpec{
				User: "alice",
				Path: "/bin/capcheck",
			}, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			process.Wait()

			Ω(buffer).Should(gbytes.Say("CAP_SYS_ADMIN: Failed to create a bind mount"))
			Ω(buffer).Should(gbytes.Say("CAP_MKNOD: Failed to make a node"))
			Ω(buffer).Should(gbytes.Say("CAP_NET_BIND_SERVICE: Create listener succeeded"))
		})
	})
})
