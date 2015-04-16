package garden_acceptance_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = PDescribe("nested containers", func() {
	var (
		gardenClient   client.Client
		outerContainer garden.Container
	)

	BeforeEach(func() {
		gardenClient = client.New(connection.New("tcp", "127.0.0.1:7777"))
		destroyAllContainers(gardenClient)

		outerContainer = createContainer(gardenClient, garden.ContainerSpec{
			RootFSPath: "/home/vagrant/garden/rootfs/nestable",
			Privileged: true,
			BindMounts: []garden.BindMount{
				{SrcPath: "/var/vcap/packages/garden-linux/bin", DstPath: "/home/vcap/bin/", Mode: garden.BindMountModeRO},
				{SrcPath: "/var/vcap/packages/garden-linux/src/github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/bin", DstPath: "/home/vcap/binpath/bin", Mode: garden.BindMountModeRO},
				{SrcPath: "/var/vcap/packages/garden-linux/src/github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/skeleton", DstPath: "/home/vcap/binpath/skeleton", Mode: garden.BindMountModeRO},
				{SrcPath: "/var/vcap/packages/busybox", DstPath: "/home/vcap/rootfs", Mode: garden.BindMountModeRO},
			},
		})

		nestedServerOutput := gbytes.NewBuffer()
		_, err := outerContainer.Run(garden.ProcessSpec{
			Path: "sh",
			User: "root",
			Dir:  "/home/vcap",
			Args: []string{
				"-c",
				`mkdir -p /tmp/overlays /tmp/containers /tmp/snapshots /tmp/graph;
				./bin/garden-linux \
					-bin /home/vcap/binpath/bin \
					-rootfs /home/vcap/rootfs \
					-depot /tmp/containers \
					-overlays /tmp/overlays \
					-snapshots /tmp/snapshots \
					-graph /tmp/graph \
					-disableQuotas \
					-listenNetwork tcp \
					-listenAddr 0.0.0.0:7778`,
			},
		}, recordedProcessIO(nestedServerOutput))
		Ω(err).ShouldNot(HaveOccurred(), "Error while running nested garden")
		Eventually(nestedServerOutput).Should(gbytes.Say("garden-linux.started"))
	})

	AfterEach(func() {
		destroyAllContainers(gardenClient)
	})

	It("can run a nested container (#83806940)", func() {
		info, err := outerContainer.Info()
		Ω(err).ShouldNot(HaveOccurred())

		stdout, stderr, err := runCommand(fmt.Sprintf("curl -sSH \"Content-Type: application/json\" -XPOST http://%s:7778/containers -d '{}'", info.ContainerIP))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stderr).Should(Equal(""), "Curl STDERR")
		Ω(stdout).Should(HavePrefix("{\"Handle\":"), "Curl STDOUT")
		Ω(gardenClient.Destroy(outerContainer.Handle())).Should(Succeed())
	})
})
