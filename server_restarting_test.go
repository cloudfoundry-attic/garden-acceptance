package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = PDescribe("When the server is restarted", func() {
	restartGarden := func() {
		stdout, _, err := runCommand("sudo /vagrant/vagrant/ctl restart")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(stdout).Should(ContainSubstring("Starting server"))
	}

	// Restarting the Garden server is expensive, so we lump all of the tests
	// into one big It statement.
	It("continues to run the containers", func() {
		By("Setup containers and restart garden.")
		container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
		container2 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
		Ω(container.NetOut(pingRule("8.8.8.8"))).Should(Succeed())

		restartGarden()

		By("Restore multiple privileged containers (#88685840)")
		_, err := container.Info()
		Ω(err).ShouldNot(HaveOccurred())
		_, err = container2.Info()
		Ω(err).ShouldNot(HaveOccurred())

		By("Run a process in a restored container (#88686146)")
		_, err = container.Run(lsProcessSpec, silentProcessIO)
		Ω(err).ShouldNot(HaveOccurred())

		By("NetOut survives restart (#82554270)")
		stdout := runInContainerSuccessfully(container, "ping -c 1 -w 3 8.8.8.8")
		Ω(stdout).Should(ContainSubstring("64 bytes from"))
		Ω(stdout).ShouldNot(ContainSubstring("100% packet loss"))
	})
})
