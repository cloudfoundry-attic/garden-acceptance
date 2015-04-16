package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("docker docker docker", func() {
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
})
