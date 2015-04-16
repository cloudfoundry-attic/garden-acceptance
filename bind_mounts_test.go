package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bind_mounts", func() {
	It("mounts a read-only BindMount (#75464648)", func() {
		runCommand("sudo rm -f /var/bindmount-test")

		container := createContainer(gardenClient, garden.ContainerSpec{
			BindMounts: []garden.BindMount{
				garden.BindMount{
					SrcPath: "/var",
					DstPath: "/home/vcap/readonly",
					Mode:    garden.BindMountModeRO},
			},
		})

		runCommand("sudo touch /var/bindmount-test")
		stdout := runInContainerSuccessfully(container, "ls -l /home/vcap/readonly")
		Ω(stdout).Should(ContainSubstring("bindmount-test"))

		_, stderr, _ := runInContainer(container, "rm /home/vcap/readonly/bindmount-test")
		Ω(stderr).Should(ContainSubstring("Read-only file system"))

		runCommand("sudo rm -f /var/bindmount-test")
	})

	It("mounts a read/write BindMount (#75464648)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			BindMounts: []garden.BindMount{
				garden.BindMount{
					SrcPath: "/home/vcap",
					DstPath: "/home/vcap/readwrite",
					Mode:    garden.BindMountModeRW,
					Origin:  garden.BindMountOriginContainer,
				},
			},
		})

		stdout := runInContainerSuccessfully(container, "ls -l /home/vcap/readwrite")
		Ω(stdout).ShouldNot(ContainSubstring("bindmount-test"))

		stdout = runInContainerSuccessfully(container, "touch /home/vcap/readwrite/bindmount-test")
		stdout = runInContainerSuccessfully(container, "ls -l /home/vcap/readwrite")
		Ω(stdout).Should(ContainSubstring("bindmount-test"))

		runCommand("sudo rm -f /var/bindmount-test")
	})
})
