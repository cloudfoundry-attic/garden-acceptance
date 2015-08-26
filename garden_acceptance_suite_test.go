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
	"github.com/cloudfoundry-incubator/garden/client/connection"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestGardenAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Acceptance Suite")
}

var gardenClient client.Client

var _ = BeforeSuite(func() {
	gardenClient = client.New(connection.New("tcp", "10.244.16.6:7777"))
})

var _ = BeforeEach(func() {
	destroyAllContainers(gardenClient)
})

var _ = AfterEach(func() {
	destroyAllContainers(gardenClient)
})

var lsProcessSpec = garden.ProcessSpec{User: "root", Path: "ls", Args: []string{"-l", "/"}}
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

func runCommand(cmd string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command("sh", "-c", cmd)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

// TODO: Make user an explicit argument, always
func runInContainer(container garden.Container, cmd string) (string, string, error) {
	info, err := container.Info()
	Ω(err).ShouldNot(HaveOccurred())
	return runCommand(fmt.Sprintf("cd %v && sudo ./bin/wsh %v", info.ContainerPath, cmd))
}

func runInContainerSuccessfully(container garden.Container, cmd string) string {
	stdout, _, err := runInContainer(container, cmd)
	Ω(err).ShouldNot(HaveOccurred())
	return stdout
}

func createContainer(client garden.Client, spec garden.ContainerSpec) garden.Container {
	container, err := client.Create(spec)
	Ω(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Error while creating container with spec: %+v", spec))
	return container
}

func destroyAllContainers(client client.Client) {
	containers, err := client.Containers(nil)
	Ω(err).ShouldNot(HaveOccurred(), "Error while listing containers")

	for _, container := range containers {
		err = client.Destroy(container.Handle())
		Ω(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Error while destroying container %+v", container.Handle()))
	}
}
