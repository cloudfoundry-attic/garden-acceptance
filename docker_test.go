package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("docker docker docker", func() {
	It("returns a helpful error message when image not found from default registry (#89007566)", func() {
		_, err := gardenClient.Create(garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/doesnotexist"})
		Ω(err.Error()).Should(ContainSubstring("could not fetch image cloudfoundry/doesnotexist from registry https://index.docker.io/v1/: HTTP code: 404"))
	})

	It("returns a helpful error message when image not found from custom registry (#89007566)", func() {
		_, err := gardenClient.Create(garden.ContainerSpec{RootFSPath: "docker://quay.io/cloudfoundry/doesnotexist"})
		Ω(err.Error()).Should(ContainSubstring("could not fetch image cloudfoundry/doesnotexist from registry quay.io"))
	})

	It("can create a container without /bin/sh (#90521974)", func() {
		_, err := gardenClient.Create(garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/no-sh"})
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("mounts an ubuntu docker image, just fine", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///onsi/grace"})
		process, err := container.Run(lsProcessSpec, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
	})

	It("mounts a non-ubuntu docker image, just fine", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///onsi/grace-busybox"})
		process, err := container.Run(lsProcessSpec, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
	})

	It("creates directories for volumes listed in VOLUME (#85482656)", func() {
		buffer := gbytes.NewBuffer()
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/with-volume"})
		process, err := container.Run(lsProcessSpec, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("foo"))
	})

	It("respects ENV vars from Dockerfile (#86540096)", func() {
		buffer := gbytes.NewBuffer()
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/with-volume"})
		process, err := container.Run(
			garden.ProcessSpec{Path: "sh", Args: []string{"-c", "echo $PATH"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("from-dockerfile"))
	})

	It("supports other registrys (#77226688)", func() {
		createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker://quay.io/tammersaleh/testing"})
	})

	It("maintains permissions from the docker image (#91955652)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/garden-pm#alice"})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(
			garden.ProcessSpec{User: "vcap", Path: "touch", Args: []string{"/home/alice/not_me"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("touch: cannot touch '/home/alice/not_me': Permission denied"))

		process, err = container.Run(
			garden.ProcessSpec{User: "alice", Path: "touch", Args: []string{"/home/alice/me"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		process, err = container.Run(
			garden.ProcessSpec{User: "root", Path: "touch", Args: []string{"/var/i_am_root"}},
			silentProcessIO,
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		buffer = gbytes.NewBuffer()
		process, err = container.Run(
			garden.ProcessSpec{User: "alice", Path: "touch", Args: []string{"/i_am_not_root"}},
			recordedProcessIO(buffer),
		)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).ShouldNot(Equal(0))
		Ω(buffer).Should(gbytes.Say("touch: cannot touch '/i_am_not_root': Permission denied"))
	})
})
