package garden_acceptance_test

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"testing"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestGardenAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Acceptance Suite")
}

var _ = BeforeSuite(func() {
	stdout, _, err := runCommand("sudo /vagrant/vagrant/ctl restart")
	Ω(err).ShouldNot(HaveOccurred())
	Ω(stdout).Should(ContainSubstring("Starting server"))
})

var _ = AfterSuite(func() {
	stdout, _, err := runCommand("sudo /vagrant/vagrant/ctl stop")
	Ω(err).ShouldNot(HaveOccurred())
	Ω(stdout).Should(ContainSubstring("Stopping server"))
})

var lsProcessSpec = garden.ProcessSpec{Path: "ls", Args: []string{"-l", "/"}}
var silentProcessIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

func recordedProcessIO(buffer *gbytes.Buffer) garden.ProcessIO {
	return garden.ProcessIO{
		Stdout: io.MultiWriter(buffer, GinkgoWriter),
		Stderr: io.MultiWriter(buffer, GinkgoWriter),
	}
}

func pingRule(ip string) garden.NetOutRule {
	return garden.NetOutRule{
		Protocol: garden.ProtocolICMP,
		Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP(ip))},
	}
}

func TCPRule(IP string, Port uint16) garden.NetOutRule {
	return garden.NetOutRule{
		Protocol: garden.ProtocolTCP,
		Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP(IP))},
		Ports:    []garden.PortRange{garden.PortRangeFromPort(Port)},
		Log:      true,
	}
}

func runCommand(cmd string) (string, string, error) {
	var stdout, stderr bytes.Buffer

	command := exec.Command("sh", "-c", cmd)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	return stdout.String(), stderr.String(), err
}

func runInContainer(container garden.Container, cmd string) (string, string, error) {
	info, _ := container.Info()
	command := fmt.Sprintf("cd %v && sudo ./bin/wsh %v", info.ContainerPath, cmd)
	return runCommand(command)
}

func runInContainerSuccessfully(container garden.Container, cmd string) string {
	stdout, _, err := runInContainer(container, cmd)
	Ω(err).ShouldNot(HaveOccurred())
	return stdout
}

func destroyAllContainers(client client.Client) {
	containers, err := client.Containers(nil)
	Ω(err).ShouldNot(HaveOccurred(), "Error while listing containers")

	for _, container := range containers {
		err = client.Destroy(container.Handle())
		Ω(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Error while destroying container %+v", container.Handle()))
	}
}

func createContainer(client garden.Client, spec garden.ContainerSpec) (container garden.Container) {
	container, err := client.Create(spec)
	Ω(err).ShouldNot(
		HaveOccurred(),
		fmt.Sprintf("Error while creating container with spec: %+v", spec))
	return
}
