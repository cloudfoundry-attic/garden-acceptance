package garden_acceptance_test

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("networking", func() {
	Describe("NetIn rules", func() {
		verifyNetIn := func(container garden.Container, hostPort, containerPort uint32) {
			_, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "sh",
				Args: []string{"-c", fmt.Sprintf("echo hello | nc -l -p %d", containerPort)},
			}, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			time.Sleep(time.Millisecond * 100)

			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", hostIP, hostPort))
			Ω(err).ShouldNot(HaveOccurred())

			message, err := bufio.NewReader(conn).ReadString('\n')
			Ω(err).ShouldNot(HaveOccurred())
			Ω(message).Should(Equal("hello\n"))
		}

		It("works when ports are provided", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})

			hostPort, containerPort := uint32(8080), uint32(9090)
			returnedHostPort, returnedContainerPort, err := container.NetIn(hostPort, containerPort)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(returnedHostPort).Should(Equal(hostPort))
			Ω(returnedContainerPort).Should(Equal(containerPort))

			verifyNetIn(container, hostPort, containerPort)
		})

		PIt("works when ports are not provided", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})

			hostPort, containerPort, err := container.NetIn(0, 0)
			Ω(err).ShouldNot(HaveOccurred())

			verifyNetIn(container, hostPort, containerPort)
		})

		// TODO: include restarting in the test, test with snapshotting
		PIt("has FIFO semantics on host side port reuse for NetIn rules", func() {
			// port_pool_size is 5 in manifest
			containerA := createContainer(gardenClient, garden.ContainerSpec{})
			containerAPort, _, err := containerA.NetIn(0, 0)
			Ω(err).ShouldNot(HaveOccurred())

			containerB := createContainer(gardenClient, garden.ContainerSpec{})
			containerBPort, _, err := containerB.NetIn(0, 0)
			Ω(err).ShouldNot(HaveOccurred())

			containerC := createContainer(gardenClient, garden.ContainerSpec{})
			for i := 0; i < 3; i++ {
				_, _, err := containerC.NetIn(0, 0)
				Ω(err).ShouldNot(HaveOccurred())
			}

			Ω(gardenClient.Destroy(containerB.Handle())).Should(Succeed())
			Ω(gardenClient.Destroy(containerA.Handle())).Should(Succeed())
			Ω(gardenClient.Destroy(containerC.Handle())).Should(Succeed())

			containerD := createContainer(gardenClient, garden.ContainerSpec{})
			firstReusedPort, _, err := containerD.NetIn(0, 0)
			Ω(err).ShouldNot(HaveOccurred())
			secondReusedPort, _, err := containerD.NetIn(0, 0)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(firstReusedPort).To(Equal(containerBPort))
			Ω(secondReusedPort).To(Equal(containerAPort))
		})
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

	// TODO: Work out how to check this on the host
	PIt("logs outbound TCP connections (#90216342, #82554270)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Handle: "Unique"})
		Ω(container.NetOut(tcpRule("93.184.216.34", 80))).Should(Succeed())

		_, _, err := runCommand("sudo sh -c 'echo > /var/log/syslog'")
		Ω(err).ShouldNot(HaveOccurred())
		stdout := runInContainerSuccessfully(container, "wget -qO- http://example.com")
		Ω(stdout).Should(ContainSubstring("Example Domain"))

		stdout, _, err = runCommand("sudo cat /var/log/syslog")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(ContainSubstring("Unique"))
		Ω(stdout).Should(ContainSubstring("DST=93.184.216.34"))
	})

	It("respects network option to set subnet for a container (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.3/24"})
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ifconfig",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))

		Ω(buffer).Should(gbytes.Say("inet addr:10.2.0.3"))
		Ω(buffer).Should(gbytes.Say("Bcast:0.0.0.0  Mask:255.255.255.0"))

		buffer = gbytes.NewBuffer()
		process, err = container.Run(garden.ProcessSpec{
			User: "root",
			Path: "route",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("default\\s+10.2.0.1"))
	})

	It("allows containers to talk to each other (#75464982)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.0/30"})
		container2 := createContainer(gardenClient, garden.ContainerSpec{Network: "10.3.0.0/30"})
		info, err := container2.Info()
		Ω(err).ShouldNot(HaveOccurred())
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ping",
			Args: []string{"-c", "1", "-w", "3", info.ContainerIP},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("64 bytes from"))
		Ω(buffer).ShouldNot(gbytes.Say("100% packet loss"))
	})

	It("doesn't destroy routes when destroying container (Bug #83656106)", func() {
		container1 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.2.0.0/24"})
		container2 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true, Network: "10.3.0.0/24"})
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
		_ = createContainer(gardenClient, garden.ContainerSpec{Network: "10.2.0.0/24"})
		_, err := gardenClient.Create(garden.ContainerSpec{Network: "10.2.0.3/16"})
		Ω(err).Should(HaveOccurred())
		Ω(err).Should(MatchError("the requested subnet (10.2.0.0/16) overlaps an existing subnet (10.2.0.0/24)"))
	})

	PIt("should allow configuration of MTU (#80221576)", func() {
		container, err := gardenClient.Create(garden.ContainerSpec{
			RootFSPath: "docker:///onsi/grace-busybox",
		})
		Ω(err).ShouldNot(HaveOccurred())

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "ifconfig",
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		Ω(buffer).Should(gbytes.Say("MTU:1499"))

		// TODO: Work out how to check on the host end
		// stdout, _, err = runCommand("/sbin/ifconfig")
		// Ω(err).ShouldNot(HaveOccurred())
		// Ω(stdout).Should(ContainSubstring("MTU:1499"))
	})

	It("container ip reuse", func() {
		containerIP := func(container garden.Container) string {
			info, err := container.Info()
			Ω(err).ShouldNot(HaveOccurred())
			return info.ContainerIP
		}

		one := createContainer(gardenClient, garden.ContainerSpec{})
		two := createContainer(gardenClient, garden.ContainerSpec{})

		ipOne := containerIP(one)
		ipTwo := containerIP(two)
		Ω(ipTwo).ShouldNot(Equal(ipOne))

		Ω(gardenClient.Destroy(one.Handle())).To(Succeed())

		three := createContainer(gardenClient, garden.ContainerSpec{})
		ipThree := containerIP(three)
		Ω(ipThree).Should(Equal(ipOne))

		hostPort, containerPort := uint32(8080), uint32(9090)
		_, _, err := three.NetIn(hostPort, containerPort)
		Ω(err).ShouldNot(HaveOccurred())

		_, err = three.Run(garden.ProcessSpec{
			User: "root",
			Path: "sh",
			Args: []string{"-c", fmt.Sprintf("echo hello | nc -l -p %d", containerPort)},
		}, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())
		time.Sleep(time.Millisecond * 100)

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", hostIP, hostPort))
		Ω(err).ShouldNot(HaveOccurred())

		message, err := bufio.NewReader(conn).ReadString('\n')
		Ω(err).ShouldNot(HaveOccurred())
		Ω(message).Should(Equal("hello\n"))
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
