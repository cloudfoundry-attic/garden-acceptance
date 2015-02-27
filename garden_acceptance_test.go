package garden_acceptance_test

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var lsProcessSpec = garden.ProcessSpec{Path: "ls", Args: []string{"-l", "/"}}
var silentProcessIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

func recordedProcessIO(buffer *gbytes.Buffer) garden.ProcessIO {
	return garden.ProcessIO{
		Stdout: io.MultiWriter(buffer, GinkgoWriter),
		Stderr: io.MultiWriter(buffer, GinkgoWriter),
	}
}

func pingRule(IP string) garden.NetOutRule {
	return garden.NetOutRule{
		Protocol: garden.ProtocolICMP,
		Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP(IP))},
	}
}

func gardenLinuxReleaseDir() (s string) {
	s = os.Getenv("GARDEN_LINUX_RELEASE_DIR")
	if s == "" {
		Fail("$GARDEN_LINUX_RELEASE_DIR must be set to the path of the garden-linux-release repo")
	}
	return
}

func myDir() (s string) {
	_, filename, _, _ := runtime.Caller(1)
	return path.Dir(filename)
}

func runInVagrant(cmd string) (string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := exec.Command("vagrant", "ssh", "-c", cmd)
	command.Dir = gardenLinuxReleaseDir()
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.Run()

	return stdout.String(), stderr.String()
}

func runInContainer(container garden.Container, cmd string) (string, string) {
	info, _ := container.Info()
	command := fmt.Sprintf("cd %v && sudo ./bin/wsh %v", info.ContainerPath, cmd)
	return runInVagrant(command)
}

func runInContainerSuccessfully(container garden.Container, cmd string) string {
	stdout, stderr := runInContainer(container, cmd)
	Ω(stderr).Should(Equal(""))
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

func installRootImage(rootfs_name string) string {
	local_path := path.Join(myDir(), "rootfs_images", rootfs_name+".tgz")
	vagrant_path := path.Join(gardenLinuxReleaseDir(), rootfs_name+".tgz")
	_ = os.Remove(vagrant_path)
	err := os.Link(local_path, vagrant_path)
	Ω(err).ShouldNot(HaveOccurred())
	rootfs_directory := "/home/vcap/" + rootfs_name

	_, stderr := runInVagrant("sudo mkdir -p " + rootfs_directory + " && sudo tar -xzf /vagrant/" + rootfs_name + ".tgz -C " + rootfs_directory)
	Ω(stderr).Should(Equal(""))
	return rootfs_directory
}

func removeRootImage(rootfs_name string) {
	err := os.Remove(path.Join(gardenLinuxReleaseDir(), rootfs_name+".tgz"))
	Ω(err).ShouldNot(HaveOccurred())
	_, stderr := runInVagrant("sudo rm -rf /home/vcap/" + rootfs_name)
	Ω(stderr).Should(Equal(""))
}

var _ = Describe("Garden Acceptance Tests", func() {
	var gardenClient client.Client

	BeforeEach(func() {
		gardenClient = client.New(connection.New("tcp", "127.0.0.1:7777"))
		destroyAllContainers(gardenClient)
	})

	Describe("when garden is running in a container,", func() {
		var outer_container garden.Container
		nestedServerOutput := gbytes.NewBuffer()

		AfterEach(func() { removeRootImage("nestable") })

		BeforeEach(func() {
			nested_rootfs_path := installRootImage("nestable")

			outer_container = createContainer(gardenClient, garden.ContainerSpec{
				RootFSPath: nested_rootfs_path,
				Privileged: true,
				BindMounts: []garden.BindMount{
					{SrcPath: "/var/vcap/packages/garden-linux/bin", DstPath: "/home/vcap/bin/", Mode: garden.BindMountModeRO},
					{SrcPath: "/var/vcap/packages/garden-linux/src/github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/bin", DstPath: "/home/vcap/binpath/bin", Mode: garden.BindMountModeRO},
					{SrcPath: "/var/vcap/packages/garden-linux/src/github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/skeleton", DstPath: "/home/vcap/binpath/skeleton", Mode: garden.BindMountModeRO},
					{SrcPath: "/var/vcap/packages/busybox", DstPath: "/home/vcap/rootfs", Mode: garden.BindMountModeRO},
				},
			})

			_, err := outer_container.Run(garden.ProcessSpec{
				Path: "sh",
				User: "root",
				Dir:  "/home/vcap",
				Args: []string{
					"-c",
					`mkdir -p /tmp/overlays /tmp/containers /tmp/snapshots /tmp/graph;
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

		PIt("can run a nested container (#83806940)", func() {
			info, err := outer_container.Info()
			Ω(err).ShouldNot(HaveOccurred())

			stdout, stderr := runInVagrant(fmt.Sprintf("curl -sSH \"Content-Type: application/json\" -XPOST http://%s:7778/containers -d '{}'", info.ContainerIP))

			Ω(stderr).Should(Equal(""), "Curl STDERR")
			Ω(stdout).Should(HavePrefix("{\"Handle\":"), "Curl STDOUT")
			Ω(gardenClient.Destroy(outer_container.Handle())).Should(Succeed())
		})
	})

	Describe("A container", func() {
		var container garden.Container

		Context("that's privileged", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
			})

			It("can set rlimits when running processes", func() {
				var nofile uint64 = 1234
				output := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
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
						process, err = container.Run(garden.ProcessSpec{Path: "mkdir", Privileged: true, Args: []string{"-p", dir}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error making "+dir)

						process, err = container.Run(garden.ProcessSpec{Path: "sh", Privileged: true, Args: []string{"-c", "echo 'readlink -f $0' > " + foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running echo on "+foo)
						Ω(process.Wait()).Should(Equal(0), "echo exited with bad error code.")

						process, err = container.Run(garden.ProcessSpec{Path: "chmod", Privileged: true, Args: []string{"+x", foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running chmod on "+foo)
						Ω(process.Wait()).Should(Equal(0), "chmod +x "+foo+" exited with bad error code.")
					}
				})

				It("sets the path correctly, when running privileged processes", func() {
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
						process, err = container.Run(garden.ProcessSpec{Path: "foo", Privileged: true}, recordedProcessIO(buffer))
						Ω(err).ShouldNot(HaveOccurred(), "Error running foo.")
						Ω(process.Wait()).Should(Equal(0), "Foo exited with bad error code.")
						Ω(string(buffer.Contents())).Should(Equal(foo + "\n"))

						process, err = container.Run(garden.ProcessSpec{Path: "rm", Privileged: true, Args: []string{foo}}, silentProcessIO)
						Ω(err).ShouldNot(HaveOccurred(), "Error running removing "+foo)
						Ω(process.Wait()).Should(Equal(0), "rm "+foo+" exited with bad error code.")
					}
				})

				It("sets the path correctly, when running unprivileged processes", func() {
					var process garden.Process
					var err error
					var foo string
					directories := []string{
						"/usr/local/bin/",
						"/usr/bin/",
						"/bin/",
					}
					// run them
					for _, dir := range directories {
						foo = dir + "foo"
						buffer := gbytes.NewBuffer()
						process, err = container.Run(garden.ProcessSpec{Path: "foo", Privileged: false}, recordedProcessIO(buffer))
						Ω(err).ShouldNot(HaveOccurred(), "Error running foo.")
						Ω(process.Wait()).Should(Equal(0), "Foo exited with bad error code.")
						Ω(string(buffer.Contents())).Should(Equal(foo + "\n"))

						process, err = container.Run(garden.ProcessSpec{Path: "rm", Privileged: true, Args: []string{foo}}, silentProcessIO)
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
				value, err := container.GetProperty("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(value).Should(Equal("bar"))

				err = container.SetProperty("foo", "baz")
				Ω(err).ShouldNot(HaveOccurred())

				err = container.SetProperty("fiz", "buz")
				Ω(err).ShouldNot(HaveOccurred())

				err = container.RemoveProperty("foo")
				Ω(err).ShouldNot(HaveOccurred())

				info, err := container.Info()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(info.Properties).Should(Equal(garden.Properties{"fiz": "buz"}))
			})
		})

		Context("without initial properties", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{})
			})

			It("can set a property (#87599106)", func() {
				err := container.SetProperty("foo", "bar")
				Ω(err).ShouldNot(HaveOccurred())

				value, err := container.GetProperty("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(value).Should(Equal("bar"))
			})
		})

		Context("that's unprivileged", func() {
			BeforeEach(func() {
				container = createContainer(gardenClient, garden.ContainerSpec{Privileged: false})
			})

			It("defaults to running processes as vcap when unpriviledged", func() {
				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{Path: "whoami", Privileged: false}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.Wait()).Should(Equal(0))
				Ω(buffer.Contents()).Should(ContainSubstring("vcap"))
			})

			PIt("can run as an arbitrary user (#82838924)", func() {
				stdout := runInContainerSuccessfully(container, "cat /etc/passwd")
				Ω(stdout).Should(ContainSubstring("anotheruser"))

				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{Path: "whoami", User: "anotheruser", Privileged: false}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.Wait()).Should(Equal(0))
				Ω(buffer.Contents()).Should(ContainSubstring("anotheruser"))
			})

			It("can send TERM and KILL signals to processes (#83231270)", func() {
				run_buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", `
						trap 'echo "TERM received"' SIGTERM
						while true; do echo waiting; sleep 1; done
					`},
				}, recordedProcessIO(run_buffer))
				Ω(err).ShouldNot(HaveOccurred())

				process_buffer := gbytes.NewBuffer()
				process, err = container.Attach(process.ID(), recordedProcessIO(process_buffer))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(process_buffer, "2s").Should(gbytes.Say("waiting"), "Process is running")

				Ω(process.Signal(garden.SignalTerminate)).Should(Succeed(), "Process sent the TERM signal")
				Eventually(process_buffer, "2s").Should(gbytes.Say("TERM received"), "Process received the TERM signal")

				Eventually(process_buffer, "2s").Should(gbytes.Say("waiting"), "Process is still running")
				Ω(process.Signal(garden.SignalKill)).Should(Succeed(), "Process being killed")
				Ω(process.Wait()).Should(Equal(255))
			})

			It("allows the process to catch SIGCHLD (#85801952)", func() {
				run_buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", `
						trap 'echo "SIGCHLD received"' SIGCHLD
						sleep 1; echo waiting;
						$(ls / >/dev/null 2>&1);
						sleep 1; echo waiting;
					`},
				}, recordedProcessIO(run_buffer))
				Ω(err).ShouldNot(HaveOccurred())

				process_buffer := gbytes.NewBuffer()
				process, err = container.Attach(process.ID(), recordedProcessIO(process_buffer))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(process_buffer, "2s").Should(gbytes.Say("SIGCHLD received"))
				Ω(process.Wait()).Should(Equal(0))
			})

			It("can run privileged processes as root", func() {
				buffer := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{Path: "whoami", Privileged: true}, recordedProcessIO(buffer))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.Wait()).Should(Equal(0))
				Ω(buffer.Contents()).Should(ContainSubstring("root"))
			})

			It("can run unprivileged processes as fake root", func() {
				stderr := gbytes.NewBuffer()
				recorder := garden.ProcessIO{Stdout: GinkgoWriter, Stderr: io.MultiWriter(stderr, GinkgoWriter)}
				process, err := container.Run(garden.ProcessSpec{Path: "cat", Args: []string{"/proc/vmallocinfo"}, User: "root", Privileged: false}, recorder)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.Wait()).ShouldNot(Equal(0))
				Ω(stderr.Contents()).Should(ContainSubstring("Permission denied"), "Stderr")
			})

			It("can run unprivileged processes with rlimits", func() {
				var nofile uint64 = 1234
				output := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
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

	Describe("Networking", func() {
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

		// It("can open outbound TCP connections (#82554270)", func() {
		// 	container := createContainer(gardenClient, garden.ContainerSpec{})
		// 	Ω(container.NetOut(tcpRule("8.8.8.8"))).ShouldNot(HaveOccurred())
		//
		// 	stdout = runInContainerSuccessfully(container, "ping -c 1 -w 3 8.8.8.8")
		// 	Ω(stdout).Should(ContainSubstring("64 bytes from"))
		// 	Ω(stdout).ShouldNot(ContainSubstring("100% packet loss"))
		// })
		//
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

	Describe("When the server is restarted", func() {
		restartGarden := func() {
			restart := "sudo /var/vcap/bosh/bin/monit restart garden"
			wait := "while sudo /var/vcap/bosh/bin/monit summary | grep \"Process 'garden'\"| grep -vq running; do sleep 1; done"
			stdout, stderr := runInVagrant(restart + " && " + wait)
			Ω(stdout).Should(Equal(""))
			Ω(stderr).Should(Equal(""))
		}

		It("continues to run the containers", func() {
			By("Setup containers and restart garden.")
			container := createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
			container2 := createContainer(gardenClient, garden.ContainerSpec{Privileged: true})
			Ω(container.NetOut(pingRule("8.8.8.8"))).ShouldNot(HaveOccurred())

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
	})

	It("supports setting environment variables on the container (#77303456)", func() {
		container := createContainer(gardenClient, garden.ContainerSpec{
			Env: []string{
				"ROOT_ENV=A",
				"OVERWRITTEN_ENV=B",
				"HOME=/nowhere",
			},
		})

		buffer := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
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

	Describe("mounting docker images", func() {
		It("fails when creating a container who's rootfs does not have /bin/sh (#77771202)", func() {
			_, err := gardenClient.Create(garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/empty"})
			Ω(err).Should(HaveOccurred())
		})

		It("mounts an ubuntu docker image, just fine", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///onsi/grace"})
			process, err := container.Run(lsProcessSpec, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
		})

		It("mounts a none-ubuntu docker image, just fine", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///onsi/grace-busybox"})
			process, err := container.Run(lsProcessSpec, silentProcessIO)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
		})

		It("creates directories for volumes listed in VOLUME (#85482656)", func() {
			buffer := gbytes.NewBuffer()
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/with-volume"})
			process, err := container.Run(lsProcessSpec, recordedProcessIO(buffer))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(buffer).Should(gbytes.Say("foo"))
		})

		It("respects ENV vars from Dockerfile (#86540096)", func() {
			buffer := gbytes.NewBuffer()
			container := createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker:///cloudfoundry/with-volume"})
			process, err := container.Run(
				garden.ProcessSpec{Path: "sh", Args: []string{"-c", "echo $PATH"}},
				recordedProcessIO(buffer),
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(buffer).Should(gbytes.Say("from-dockerfile"))
		})

		It("supports other registrys (#77226688)", func() {
			createContainer(gardenClient, garden.ContainerSpec{RootFSPath: "docker://quay.io/tammersaleh/testing"})
		})
	})

	Describe("Fusefs", func() {
		var fusefs_rootfs_path string
		var container garden.Container

		AfterEach(func() {
			destroyAllContainers(gardenClient)
			removeRootImage("fusefs")
		})

		BeforeEach(func() {
			fusefs_rootfs_path = installRootImage("fusefs")
		})

		It("can mount the fusefs", func() {
			container = createContainer(gardenClient, garden.ContainerSpec{Privileged: true, RootFSPath: fusefs_rootfs_path})
			mountpoint := "/tmp/fuse-test"
			output := gbytes.NewBuffer()

			process, err := container.Run(garden.ProcessSpec{Path: "mkdir", Args: []string{"-p", mountpoint}}, recordedProcessIO(output))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0), "Could not make temporary directory!")

			output = gbytes.NewBuffer()
			process, err = container.Run(garden.ProcessSpec{Privileged: true, Path: "/usr/bin/hellofs", Args: []string{mountpoint}}, recordedProcessIO(output))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0), "Failed to mount hello filesystem.")

			output = gbytes.NewBuffer()
			process, err = container.Run(garden.ProcessSpec{Privileged: true, Path: "cat", Args: []string{mountpoint + "/hello"}}, recordedProcessIO(output))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0), "Failed to find hello file.")
			Ω(output).Should(gbytes.Say("Hello World!"))

			output = gbytes.NewBuffer()
			process, err = container.Run(garden.ProcessSpec{Privileged: true, Path: "fusermount", Args: []string{"-u", mountpoint}}, recordedProcessIO(output))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0), "Failed to unmount user filesystem.")

			output = gbytes.NewBuffer()
			process, err = container.Run(garden.ProcessSpec{Privileged: true, Path: "ls", Args: []string{mountpoint}}, recordedProcessIO(output))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(output).ShouldNot(gbytes.Say("hello"), "Fuse filesystem appears still to be visible after being unmounted.")
		})
	})

	Describe("BindMounts", func() {
		It("mounts a read-only BindMount (#75464648)", func() {
			runInVagrant("/usr/bin/sudo rm -f /var/bindmount-test")

			container := createContainer(gardenClient, garden.ContainerSpec{
				BindMounts: []garden.BindMount{
					garden.BindMount{
						SrcPath: "/var",
						DstPath: "/home/vcap/readonly",
						Mode:    garden.BindMountModeRO},
				},
			})

			runInVagrant("sudo touch /var/bindmount-test")
			stdout := runInContainerSuccessfully(container, "ls -l /home/vcap/readonly")
			Ω(stdout).Should(ContainSubstring("bindmount-test"))

			stdout, stderr := runInContainer(container, "rm /home/vcap/readonly/bindmount-test")
			Ω(stderr).Should(ContainSubstring("Read-only file system"))

			runInVagrant("sudo rm -f /var/bindmount-test")
		})

		It("mounts a read/write BindMount (#75464648)", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{
				BindMounts: []garden.BindMount{
					garden.BindMount{
						SrcPath: "/home/vcap",
						DstPath: "/home/vcap/readwrite",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginContainer,
					},
				},
			})

			stdout := runInContainerSuccessfully(container, "ls -l /home/vcap/readwrite")
			Ω(stdout).ShouldNot(ContainSubstring("bindmount-test"))

			stdout = runInContainerSuccessfully(container, "touch /home/vcap/readwrite/bindmount-test")
			stdout = runInContainerSuccessfully(container, "ls -l /home/vcap/readwrite")
			Ω(stdout).Should(ContainSubstring("bindmount-test"))

			runInVagrant("sudo rm -f /var/bindmount-test")
		})
	})

})
