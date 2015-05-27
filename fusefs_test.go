package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("fusefs", func() {
	It("can be mounted", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, RootFSPath: "/home/vagrant/garden/rootfs/fusefs"})
		mountpoint := "/tmp/fuse-test"

		process, err := container.Run(garden.ProcessSpec{User: "root", Path: "mkdir", Args: []string{"-p", mountpoint}}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0), "Could not make temporary directory!")

		process, err = container.Run(garden.ProcessSpec{User: "root", Path: "/usr/bin/hellofs", Args: []string{mountpoint}}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0), "Failed to mount hello filesystem.")

		output := gbytes.NewBuffer()
		process, err = container.Run(garden.ProcessSpec{User: "root", Path: "cat", Args: []string{mountpoint + "/hello"}}, recordedProcessIO(output))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0), "Failed to find hello file.")
		Ω(output).Should(gbytes.Say("Hello World!"))

		process, err = container.Run(garden.ProcessSpec{User: "root", Path: "fusermount", Args: []string{"-u", mountpoint}}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0), "Failed to unmount user filesystem.")

		output = gbytes.NewBuffer()
		process, err = container.Run(garden.ProcessSpec{User: "root", Path: "ls", Args: []string{mountpoint}}, recordedProcessIO(output))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(output).ShouldNot(gbytes.Say("hello"), "Fuse filesystem appears still to be visible after being unmounted.")
	})
})
