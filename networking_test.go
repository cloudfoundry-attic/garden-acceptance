package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("networking", func() {
	var gardenClient client.Client

	BeforeEach(func() {
		gardenClient = client.New(connection.New("tcp", "127.0.0.1:7777"))
		destroyAllContainers(gardenClient)
	})

	AfterEach(func() {
		destroyAllContainers(gardenClient)
	})

	It("gives a better error message when NetOut is given port and no protocol (#87201436)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		err := container.NetOut(garden.NetOutRule{
			Ports: []garden.PortRange{garden.PortRangeFromPort(80)},
		})
		Ω(err).Should(MatchError("Ports cannot be specified for Protocol ALL"))
	})

	It("can open outbound ICMP connections (#85601268)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		Ω(container.NetOut(pingRule("8.8.8.8"))).ShouldNot(HaveOccurred())

		stdout := runInContainerSuccessfully(container, "ping -c 1 -w 3 8.8.8.8")
		Ω(stdout).Should(ContainSubstring("64 bytes from"))
		Ω(stdout).ShouldNot(ContainSubstring("100% packet loss"))
	})

	It("logs outbound TCP connections (#90216342, #82554270)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Handle: "Unique"})
		Ω(container.NetOut(TCPRule("93.184.216.34", 80))).ShouldNot(HaveOccurred())

		_, _, err := runCommand("sudo sh -c 'echo > /var/log/syslog'")
		Ω(err).ShouldNot(HaveOccurred())
		stdout := runInContainerSuccessfully(container, "curl -s http://example.com -o -")
		Ω(stdout).Should(ContainSubstring("Example Domain"))

		stdout, _, err = runCommand("sudo cat /var/log/syslog")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(ContainSubstring("Unique"))
		Ω(stdout).Should(ContainSubstring("DST=93.184.216.34"))
	})

	It("respects network option to set default ip for a container (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.0/30"})

		stdout := runInContainerSuccessfully(container, "ifconfig")
		Ω(stdout).Should(ContainSubstring("inet addr:10.2.0.1"))
		Ω(stdout).Should(ContainSubstring("Bcast:0.0.0.0  Mask:255.255.255.252"))

		stdout = runInContainerSuccessfully(container, "route | grep default")
		Ω(stdout).Should(ContainSubstring("10.2.0.2"))
	})

	It("allows containers to talk to each other (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.1/24"})
		_ = createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.2/24"})

		stdout := runInContainerSuccessfully(container, "ping -c 1 -w 3 10.2.0.2")
		Ω(stdout).Should(ContainSubstring("64 bytes from"))
		Ω(stdout).ShouldNot(ContainSubstring("100% packet loss"))
	})

	It("doesn't destroy routes when destroying container (Bug #83656106)", func() {
		container1 := createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.1/24"})
		container2 := createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.2/24"})
		Ω(container2.NetOut(pingRule("8.8.8.8"))).ShouldNot(HaveOccurred())

		gardenClient.Destroy(container1.Handle())

		stdout := runInContainerSuccessfully(container2, "ping -c 1 -w 3 8.8.8.8")
		Ω(stdout).Should(ContainSubstring("64 bytes from"))
		Ω(stdout).ShouldNot(ContainSubstring("100% packet loss"))
	})

	It("errors gracefully when provisioning overlapping networks (#79933424)", func() {
		_ = createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.1/24"})
		_, err := gardenClient.Create(garden.ContainerSpec{Network: "10.2.0.2/16"})
		Ω(err).Should(HaveOccurred())
		Ω(err).Should(MatchError("the requested subnet (10.2.0.0/16) overlaps an existing subnet (10.2.0.0/24)"))
	})
})
