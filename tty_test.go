package garden_acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("tty", func() {
	It("allows the wondow size to be set in the ProcessSpec", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "/var/vcap/packages/rootfs/cflinuxfs2"})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			Path: "bash",
			Args: []string{"-c", "stty -a | grep rows"},
			TTY: &garden.TTYSpec{WindowSize: &garden.WindowSize{
				Rows:    40,
				Columns: 80,
			}},
		}, recordedProcessIO(buffer))
		Ω(err).NotTo(HaveOccurred())
		process.Wait()

		Expect(buffer).To(gbytes.Say("rows 40; columns 80"))
	})

	It("allows the window size to be updated", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "/var/vcap/packages/rootfs/cflinuxfs2"})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			Path: "bash",
			Args: []string{"-c", "while true; do stty -a | grep rows; sleep 1; done"},
			TTY: &garden.TTYSpec{WindowSize: &garden.WindowSize{
				Rows:    40,
				Columns: 80,
			}},
		}, recordedProcessIO(buffer))
		Ω(err).NotTo(HaveOccurred())

		Ω(process.SetTTY(garden.TTYSpec{&garden.WindowSize{Rows: 30, Columns: 70}})).To(Succeed())

		Eventually(buffer).Should(gbytes.Say("rows 30; columns 70"))

		process.Signal(garden.SignalKill)
	})
})
