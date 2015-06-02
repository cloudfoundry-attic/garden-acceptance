package garden_acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Garden Acceptance Tests", func() {
	Describe("a container", func() {
		var container garden.Container

		It("can be run with an (essentially) empty rootfs (#91423716)", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "/vagrant/rootfs/empty"})
			buffer := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{User: "root", Path: "/hello"}, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(buffer).Should(gbytes.Say("hello"))
		})

		Context("that's privileged", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
			})

			It("can set rlimits when running processes", func() {
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

			Context("and a full set of executables", func() {
				var process garden.Process
				var err error
				var foo string

				BeforeEach(func() {
					directories := []string{
						"/usr/local/sbin/",
						"/usr/local/bin/",
						"/usr/sbin/",
						"/usr/bin/",
						"/sbin/",
						"/bin/",
					}

					for _, dir := range directories {
						foo = dir + "foo"
						process, err = container.Run(garden.ProcessSpec{User: "root", Path: "mkdir", Args: []string{"-p", dir}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error making "+dir)

						process, err = container.Run(garden.ProcessSpec{User: "root", Path: "sh", Args: []string{"-c", "echo 'readlink -f $0' > " + foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running echo on "+foo)
						Ω(process.Wait()).Should(Equal(0), "echo exited with bad error code.")

						process, err = container.Run(garden.ProcessSpec{User: "root", Path: "chmod", Args: []string{"+x", foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running chmod on "+foo)
						Ω(process.Wait()).Should(Equal(0), "chmod +x "+foo+" exited with bad error code.")
					}
				})

				It("sets the path correctly", func() {
					var process garden.Process
					var err error
					var foo string
					directories := []string{
						"/usr/local/sbin/",
						"/usr/local/bin/",
						"/usr/sbin/",
						"/usr/bin/",
						"/sbin/",
						"/bin/",
					}
					// run them
					for _, dir := range directories {
						foo = dir + "foo"
						buffer := gbytes.NewBuffer()
						process, err = container.Run(garden.ProcessSpec{User: "root", Path: "foo"}, recordedProcessIO(buffer))
						Ω(err).ShouldNot(HaveOccurred(), "Error running foo.")
						Ω(process.Wait()).Should(Equal(0), "Foo exited with bad error code.")
						Ω(string(buffer.Contents())).Should(Equal(foo + "\n"))

						process, err = container.Run(garden.ProcessSpec{User: "root", Path: "rm", Args: []string{foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running removing "+foo)
						Ω(process.Wait()).Should(Equal(0), "rm "+foo+" exited with bad error code.")
					}
				})
			})
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

				err = container.SetProperty("fiz", "buz")
				Ω(err).ShouldNot(HaveOccurred())

				err = container.RemoveProperty("foo")
				Ω(err).ShouldNot(HaveOccurred())

				properties, err := container.Properties()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(properties).Should(Equal(garden.Properties{"fiz": "buz"}))
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

			It("allows containers to be destroyed when wshd isn't running", func() {
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
						while true; do echo waiting; sleep 1; done
					`},
				}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(process.Signal(garden.SignalTerminate)).Should(Succeed())
				Eventually(buffer, "2s").Should(gbytes.Say("TERM received"), "Process did not receive TERM")

				Eventually(buffer, "2s").Should(gbytes.Say("waiting"), "Process is still running")
				Ω(process.Signal(garden.SignalKill)).Should(Succeed(), "Process being killed")
				Ω(process.Wait()).Should(Equal(255))
			})

			It("avoids a TERM race condition (#89972162)", func(done Done) {
				for i := 0; i < 50; i++ {
					process, err := container.Run(garden.ProcessSpec{
						User: "root",
						Path: "sh",
						Args: []string{"-c", `while true; do echo -n "x"; sleep 1; done`},
					}, silentProcessIO)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(process.Signal(garden.SignalKill)).Should(Succeed())
					Ω(process.Wait()).Should(Equal(255))
				}
				close(done)
			}, 20.0)

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

	Describe("iodaemon", func() {
		It("supports a timeout when the process fails to link (#77842604)", func() {
			iodaemon := "/home/vagrant/go/src/github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/skeleton/bin/iodaemon"
			stdout, _, err := runCommand("timeout 3s " + iodaemon + " -timeout 1s spawn /tmp/socketPath bash -c cat <&0; echo $?")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(stdout).NotTo(ContainSubstring("124"), "124 means `timeout` timed out.")
		})
	})

	It("cleans up after running processes (#89969450)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{})
		var err error
		for i := 0; i < 10; i++ {
			_, err = container.Run(lsProcessSpec, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
		}

		info, err := container.Info()
		Ω(err).ShouldNot(HaveOccurred())
		processesPath := info.ContainerPath + "/processes"

		_, stderr, _ := runCommand("cd " + processesPath + " && ls *.sock")

		Ω(stderr).Should(ContainSubstring("No such file or directory"))
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

		It("fails when attempting to delete a non-existant container (#86044470)", func() {
			err := gardenClient.Destroy("asdf")
			Ω(err).Should(MatchError(garden.ContainerNotFoundError{Handle: "asdf"}))
		})

		It("does not leak network namespaces (Bug #91423716)", func() {
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

	It("supports setting environment variables on the container (#77303456)", func() {
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
			User: "vcap",
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
		Ω(buffer.Contents()).Should(ContainSubstring("HOME=/home/vcap"))
		Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/nowhere"))
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
			Ω(err).Should(HaveOccurred(), "Expected an error when creating a Garden container with an existing handle")
		})
	})
})
