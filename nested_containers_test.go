package garden_acceptance_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("nested containers", func() {
	var outerContainer garden.Container

	BeforeEach(func() {
		outerContainer = createContainer(gardenClient, garden.ContainerSpec{
			RootFSPath: "/home/vagrant/garden/rootfs/nestable",
			Privileged: true,
			BindMounts: []garden.BindMount{
				{SrcPath: "/home/vagrant/garden/bin", DstPath: "/home/vcap/bin/"},
				{SrcPath: "/home/vagrant/garden/libexec", DstPath: "/home/vcap/binpath/bin"},
				{SrcPath: "/home/vagrant/garden/skeleton", DstPath: "/home/vcap/binpath/skeleton"},
				{SrcPath: "/home/vagrant/garden/rootfs/nestable", DstPath: "/home/vcap/rootfs"},
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
				mount -t tmpfs tmpfs /tmp/containers;
				mount -t tmpfs tmpfs /tmp/overlays;
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

	It("can run a nested container (#83806940)", func() {
		info, err := outerContainer.Info()
		Ω(err).ShouldNot(HaveOccurred())

		stdout, _, err := runCommand(fmt.Sprintf(`curl -sSH "Content-Type: application/json" -XPOST http://%s:7778/containers -d '{}'`, info.ContainerIP))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(HavePrefix("{\"Handle\":"))
		Ω(gardenClient.Destroy(outerContainer.Handle())).Should(Succeed())
	})
})
