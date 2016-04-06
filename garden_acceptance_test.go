package garden_acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Garden Acceptance Tests", func() {
	Describe("a container", func() {
		var container garden.Container

		PIt("can be run with an (essentially) empty rootfs (#91423716)", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "/var/vcap/packages/rootfs/empty"})
			buffer := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{User: "root", Path: "/hello"}, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(buffer).Should(gbytes.Say("hello"))
		})

		Context("with initial properties", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{
					Properties: garden.Properties{"foo": "bar"},
				})
			})

			It("can CRUD properties", func() {
				value, err := container.Property("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(value).Should(Equal("bar"))

				err = container.SetProperty("foo", "baz")
				Ω(err).ShouldNot(HaveOccurred())
				value, err = container.Property("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(value).Should(Equal("baz"))

				err = container.SetProperty("fiz", "buz")
				Ω(err).ShouldNot(HaveOccurred())

				err = container.RemoveProperty("foo")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = container.Property("foo")
				Ω(err.Error()).Should(ContainSubstring("cannot Get %s:foo", container.Handle()))

				_, err = container.Property("bar")
				Ω(err.Error()).Should(ContainSubstring("cannot Get %s:bar", container.Handle()))

				properties, err := container.Properties()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(properties).Should(ContainElement("buz"))
			})

			It("can filter containers by property", func() {
				createContainer(gardenClient, garden.ContainerSpec{
					Properties: garden.Properties{"foo": "othercontainer"},
				})
				createContainer(gardenClient, garden.ContainerSpec{})

				containers, err := gardenClient.Containers(map[string]string{"foo": "bar"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(containers).Should(ConsistOf(container))
			})
		})

		Context("without initial properties", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{})
			})

			It("can set a property (#87599106)", func() {
				err := container.SetProperty("foo", "bar")
				Ω(err).ShouldNot(HaveOccurred())

				value, err := container.Property("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(value).Should(Equal("bar"))
			})
		})

		Context("that's unprivileged", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{Privileged: false})
			})

			PIt("allows containers to be destroyed when wshd isn't running", func() {
				info, _ := container.Info()
				pidFile, err := os.Open(filepath.Join(info.ContainerPath, "run", "wshd.pid"))
				Ω(err).ShouldNot(HaveOccurred())

				var pid int
				_, err = fmt.Fscanf(pidFile, "%d", &pid)
				Ω(err).ShouldNot(HaveOccurred())

				_, _, err = runCommand("sudo kill -9 " + strconv.Itoa(pid))
				Ω(err).ShouldNot(HaveOccurred())

				err = gardenClient.Destroy(container.Handle())
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("can send TERM and KILL signals to processes (#83231270)", func() {
				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "root",
					Path: "sh",
					Args: []string{"-c", `
						trap 'echo "TERM received"' TERM
						echo trapping
						while true; do echo waiting; sleep 1; done
					`},
				}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(buffer, "3s").Should(gbytes.Say("trapping"), "Process didn't report trapping")

				Ω(process.Signal(garden.SignalTerminate)).Should(Succeed())
				Eventually(buffer, "3s").Should(gbytes.Say("TERM received"), "Process did not receive TERM")

				Eventually(buffer, "3s").Should(gbytes.Say("waiting"), "Process isn't still running")
				Ω(process.Signal(garden.SignalKill)).Should(Succeed())
				Ω(process.Wait()).ShouldNot(Equal(0))
			})

			It("avoids a TERM race condition (#89972162)", func() {
				for i := 0; i < 50; i++ {
					process, err := container.Run(garden.ProcessSpec{
						User: "root",
						Path: "sh",
						Args: []string{"-c", `while true; do echo -n "x"; sleep 1; done`},
					}, silentProcessIO)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(process.Signal(garden.SignalKill)).Should(Succeed())
					Ω(process.Wait()).ShouldNot(Equal(0))
				}
			})

			It("allows the process to catch SIGCHLD (#85801952)", func() {
				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "root",
					Path: "sh",
					Args: []string{"-c", `
						trap 'echo "SIGCHLD received"' CHLD
						$(ls / >/dev/null 2>&1);
						while true; do sleep 1; echo waiting; done
					`},
				}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(buffer, "2s").Should(gbytes.Say("SIGCHLD received"))
				Ω(process.Signal(garden.SignalKill)).Should(Succeed(), "Process being killed")
				process.Wait()
			})

			It("can run processes as root", func() {
				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{User: "root", Path: "whoami"}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.Wait()).Should(Equal(0))
				Ω(buffer.Contents()).Should(ContainSubstring("root"))
			})

			It("can run processes with rlimits", func() {
				var nofile uint64 = 1234
				output := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User:   "root",
					Path:   "sh",
					Args:   []string{"-c", "ulimit -n"},
					Limits: garden.ResourceLimits{Nofile: &nofile},
				}, recordedProcessIO(output))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(output).Should(gbytes.Say("1234"))
				Ω(process.Wait()).Should(Equal(0))
			})
		})
	})

	It("cleans up after running processes (#89969450)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		for i := 0; i < 10; i++ {
			_, err := container.Run(lsProcessSpec, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
		}

		info, err := container.Info()
		Ω(err).ShouldNot(HaveOccurred())
		processesPath := info.ContainerPath + "/processes"

		Eventually(func() string {
			_, stderr, _ := runCommand("cd " + processesPath + " && ls *.sock")
			return stderr
		}).Should(ContainSubstring("No such file or directory"))
	})

	Describe("Destroy", func() {
		It("fails when attempting to delete a container twice (#76616270)", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})

			var errors = make(chan error)
			go func() { errors <- gardenClient.Destroy(container.Handle()) }()
			go func() { errors <- gardenClient.Destroy(container.Handle()) }()

			results := []error{<-errors, <-errors}

			Ω(results).Should(ConsistOf(BeNil(), HaveOccurred()))
		})

		PIt("fails when attempting to delete a non-existant container (#86044470)", func() {
			err := gardenClient.Destroy("asdf")
			Ω(err).Should(MatchError(garden.ContainerNotFoundError{Handle: "asdf"}))
		})

		PIt("does not leak network namespaces (Bug #91423716)", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})
			info, err := container.Info()
			Ω(err).ShouldNot(HaveOccurred())

			pidFile, err := os.Open(filepath.Join(info.ContainerPath, "run", "wshd.pid"))
			Ω(err).ShouldNot(HaveOccurred())

			var pid int
			_, err = fmt.Fscanf(pidFile, "%d", &pid)
			Ω(err).ShouldNot(HaveOccurred())

			err = gardenClient.Destroy(container.Handle())
			Ω(err).ShouldNot(HaveOccurred())

			stdout, _, err := runCommand("ip netns list")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(stdout).ShouldNot(ContainSubstring(strconv.Itoa(pid)))
		})
	})

	It("supports setting environment variables on the container (Diego: #77303456, Garden: #96893340)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			Env: []string{
				"ROOT_ENV=A",
				"OVERWRITTEN_ENV=B",
				"HOME=/nowhere",
				"PASSWORD=;$*@='\"$(pwd)!!",
			},
		})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			User: "root",
			Path: "sh",
			Args: []string{"-c", "printenv"},
			Env: []string{
				"OVERWRITTEN_ENV=C",
			},
		}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())

		Ω(process.Wait()).Should(Equal(0))

		Ω(buffer.Contents()).Should(ContainSubstring("OVERWRITTEN_ENV=C"))
		Ω(buffer.Contents()).ShouldNot(ContainSubstring("OVERWRITTEN_ENV=B"))
		Ω(buffer.Contents()).Should(ContainSubstring("HOME=/nowhere"))
		Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/home/root"))
		Ω(buffer.Contents()).Should(ContainSubstring("ROOT_ENV=A"))
		Ω(buffer.Contents()).Should(ContainSubstring("PASSWORD=;$*@='\"$(pwd)!!"))
	})

	Describe("Bugs around the container lifecycle (#77768828)", func() {
		It("supports deleting a container after an errant delete", func() {
			handle := fmt.Sprintf("%d", time.Now().UnixNano())

			err := gardenClient.Destroy(handle)
			Ω(err).Should(HaveOccurred())

			_, err = gardenClient.Create(garden.ContainerSpec{Handle: handle})
			Ω(err).ShouldNot(HaveOccurred())

			_, err = gardenClient.Lookup(handle)
			Ω(err).ShouldNot(HaveOccurred())

			err = gardenClient.Destroy(handle)
			Ω(err).ShouldNot(HaveOccurred(), "Expected no error when attempting to destroy this container")

			_, err = gardenClient.Lookup(handle)
			Ω(err).Should(HaveOccurred())
		})

		It("does not allow creating an already existing container", func() {
			container, err := gardenClient.Create(garden.ContainerSpec{})
			Ω(err).ShouldNot(HaveOccurred())
			_, err = gardenClient.Create(garden.ContainerSpec{Handle: container.Handle()})
			Ω(err.Error()).Should(MatchRegexp(`Handle '[\w-]+' already in use`))
		})
	})

	PIt("mounts /proc read-only", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{User: "root", Path: "cat", Args: []string{"/proc/mounts"}}, recordedProcessIO(buffer))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(process.Wait()).Should(Equal(0))
		lines := strings.Split(string(buffer.Contents()), "\n")
		for _, line := range lines {
			if strings.Contains(line, "proc") {
				Ω(line).Should(ContainSubstring("ro"))
				Ω(line).ShouldNot(ContainSubstring("rw"))
			}
		}
	})
})
