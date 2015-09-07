package garden_acceptance_test

import (
	"net"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("networking", func() {
	It("gives a better error message when NetOut is given port and no protocol (#87201436)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		err := container.NetOut(garden.NetOutRule{
			Ports: []garden.PortRange{garden.PortRangeFromPort(80)},
		})
		Ω(err).Should(MatchError("Ports cannot be specified for Protocol ALL"))
	})

	It("can open outbound ICMP connections (#85601268)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		Ω(container.NetOut(pingRule("8.8.8.8"))).Should(Succeed())
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ping",
			Args: []string{"-c", "1", "-w", "3", "8.8.8.8"},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		Ω(buffer).Should(gbytes.Say("64 bytes from"))
		Ω(buffer).ShouldNot(gbytes.Say("100% packet loss"))
	})

	It("respects network option to set default ip for a container (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.0/30"})
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ifconfig",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		Ω(buffer).Should(gbytes.Say("inet addr:10.2.0.1"))
		Ω(buffer).Should(gbytes.Say("Bcast:0.0.0.0  Mask:255.255.255.252"))

		buffer = gbytes.NewBuffer()
		process, err = container.Run(garden.ProcessSpec{
			User: "root",
			Path: "route",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("default\\s+10.2.0.2"))
	})

	It("allows containers to talk to each other (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.1/24"})
		_ = createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.2/24"})
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ping",
			Args: []string{"-c", "1", "-w", "3", "10.2.0.2"},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("64 bytes from"))
		Ω(buffer).ShouldNot(gbytes.Say("100% packet loss"))
	})

	It("doesn't destroy routes when destroying container (Bug #83656106)", func() {
		container1 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.1/24"})
		container2 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.2/24"})
		Ω(container2.NetOut(pingRule("8.8.8.8"))).Should(Succeed())

		gardenClient.Destroy(container1.Handle())

		buffer := gbytes.NewBuffer()
		process, err := container2.Run(garden.ProcessSpec{
			User: "root",
			Path: "ping",
			Args: []string{"-c", "1", "-w", "3", "8.8.8.8"},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		Ω(buffer).Should(gbytes.Say("64 bytes from"))
		Ω(buffer).ShouldNot(gbytes.Say("100% packet loss"))
	})

	It("errors gracefully when provisioning overlapping networks (#79933424)", func() {
		_ = createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.1/24"})
		_, err := gardenClient.Create(garden.ContainerSpec{Network: "10.2.0.2/16"})
		Ω(err).Should(HaveOccurred())
		Ω(err).Should(MatchError("the requested subnet (10.2.0.0/16) overlaps an existing subnet (10.2.0.0/24)"))
	})

	PIt("should allow configuration of MTU (#80221576)", func() {
		container, err := gardenClient.Create(garden.ContainerSpec{
			RootFSPath: "docker:///onsi/grace-busybox",
		})
		Ω(err).ShouldNot(HaveOccurred())

		// -mtu=1499 is set in the garden-linux start script
		stdout, _, err := runInContainer(container, "/sbin/ifconfig")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(ContainSubstring("MTU:1499"))
		stdout, _, err = runCommand("/sbin/ifconfig")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(ContainSubstring("MTU:1499"))
	})
})

func tcpRule(ip string, port uint16) garden.NetOutRule {
	return garden.NetOutRule{
		Protocol: garden.ProtocolTCP,
		Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP(ip))},
		Ports:    []garden.PortRange{garden.PortRangeFromPort(port)},
		Log:      true,
	}
}
