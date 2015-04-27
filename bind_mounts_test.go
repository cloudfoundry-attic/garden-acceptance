package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bind_mounts", func() {
	It("can mount a read-only BindMount (#75464648)", func() {
		preExistingFile := "old"
		runCommand("sudo touch /var/" + preExistingFile)
		newFile := "new"
		runCommand("sudo rm -f /var/" + newFile)

		container := createContainer(gardenClient, garden.ContainerSpec{
			BindMounts: []garden.BindMount{
				garden.BindMount{
					SrcPath: "/var",
					DstPath: "/home/vcap/readonly",
					Mode:    garden.BindMountModeRO,
				},
			},
		})
		runCommand("sudo touch /var/" + newFile)

		stdout := runInContainerSuccessfully(container, "ls /home/vcap/readonly")
		Ω(stdout).Should(ContainSubstring(preExistingFile))
		Ω(stdout).Should(ContainSubstring(newFile))

		_, stderr, _ := runInContainer(container, "rm /home/vcap/readonly/"+preExistingFile)
		Ω(stderr).Should(ContainSubstring("Read-only file system"))
		_, stderr, _ = runInContainer(container, "rm /home/vcap/readonly/"+newFile)
		Ω(stderr).Should(ContainSubstring("Read-only file system"))
	})

	It("can mount a read/write BindMount (#75464648)", func() {
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

		stdout := runInContainerSuccessfully(container, "ls /home/vcap/readwrite")
		Ω(stdout).ShouldNot(ContainSubstring("new_file"))
		runInContainerSuccessfully(container, "touch /home/vcap/readwrite/new_file")
		stdout = runInContainerSuccessfully(container, "ls /home/vcap/readwrite")
		Ω(stdout).Should(ContainSubstring("new_file"))
	})
})
