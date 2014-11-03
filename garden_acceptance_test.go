package garden_acceptance_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/cloudfoundry-incubator/garden/api"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func uniqueHandle() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

var lsProcessSpec = api.ProcessSpec{Path: "ls"}
var silentProcessIO = api.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

func recordedProcessIO(buffer *gbytes.Buffer) api.ProcessIO {
	return api.ProcessIO{
		Stdout: io.MultiWriter(buffer, GinkgoWriter),
		Stderr: io.MultiWriter(buffer, GinkgoWriter),
	}
}

func runInVagrant(cmd string) string {
	glrDir := os.Getenv("GARDEN_LINUX_RELEASE_DIR")
	if glrDir == "" {
		Fail("$GARDEN_LINUX_RELEASE_DIR must be set to the path of the garden-linux-release repo")
	}
	command := exec.Command("vagrant", "ssh", "-c", cmd)
	command.Dir = glrDir
	output, err := command.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("vagrant command %+v output unavailable: %s", command, err))
	}
	return string(output)
}

var _ = Describe("Garden Acceptance Tests", func() {
	var gardenClient client.Client

	BeforeEach(func() {
		conn := connection.New("tcp", "127.0.0.1:7777")
		gardenClient = client.New(conn)
		// Clean up after any previous test failure.
		containers, err := gardenClient.Containers(nil)
		Ω(err).ShouldNot(HaveOccurred())

		for _, container := range containers {
			gardenClient.Destroy(container.Handle())
		}
	})

	AfterEach(func() {
		containers, err := gardenClient.Containers(nil)
		Ω(err).ShouldNot(HaveOccurred())
		for _, container := range containers {
			gardenClient.Destroy(container.Handle())
		}
	})

	Describe("Networking", func() {
		It("should respect network option to set specific ip for a container (#75464982)", func() {
			handle := uniqueHandle()
			container, err := gardenClient.Create(api.ContainerSpec{
				Handle:     handle,
				Network:    "10.2.0.0/30",
				RootFSPath: "docker:///onsi/grace-busybox",
			})

			Ω(err).ShouldNot(HaveOccurred())

			info, _ := container.Info()
			output := runInVagrant(fmt.Sprintf("cd %v && sudo ./bin/wsh /sbin/ifconfig", info.ContainerPath))
			Ω(output).Should(ContainSubstring("inet addr:10.2.0."))
			Ω(output).Should(ContainSubstring("Bcast:0.0.0.0  Mask:255.255.255.252"))
		})
	})

	Describe("things that now work", func() {
		It("should fail when attempting to delete a container twice (#76616270)", func() {
			_, err := gardenClient.Create(api.ContainerSpec{Handle: "my-fun-handle"})
			Ω(err).ShouldNot(HaveOccurred())

			var errors = make(chan error)
			go func() {
				errors <- gardenClient.Destroy("my-fun-handle")
			}()
			go func() {
				errors <- gardenClient.Destroy("my-fun-handle")
			}()

			results := []error{
				<-errors,
				<-errors,
			}

			Ω(results).Should(ConsistOf(BeNil(), HaveOccurred()))
		})

		It("should support setting environment variables on the container (#77303456)", func() {
			container, err := gardenClient.Create(api.ContainerSpec{
				Handle: "cap'n-planet",
				Env: []string{
					"ROOT_ENV=A",
					"OVERWRITTEN_ENV=B",
					"HOME=/nowhere",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := gbytes.NewBuffer()
			process, err := container.Run(api.ProcessSpec{
				Path: "sh",
				Args: []string{"-c", "printenv"},
				Env: []string{
					"OVERWRITTEN_ENV=C",
				},
			}, recordedProcessIO(buffer))

			Ω(err).ShouldNot(HaveOccurred())

			process.Wait()

			gardenClient.Destroy("cap'n-planet")

			Ω(buffer.Contents()).Should(ContainSubstring("OVERWRITTEN_ENV=C"))
			Ω(buffer.Contents()).ShouldNot(ContainSubstring("OVERWRITTEN_ENV=B"))
			Ω(buffer.Contents()).Should(ContainSubstring("HOME=/home/vcap"))
			Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/nowhere"))
			Ω(buffer.Contents()).Should(ContainSubstring("ROOT_ENV=A"))
		})

		It("should fail when creating a container who's rootfs does not have /bin/sh (#77771202)", func() {
			handle := uniqueHandle()
			_, err := gardenClient.Create(api.ContainerSpec{
				Handle:     handle,
				RootFSPath: "docker:///cloudfoundry/empty",
			})
			Ω(err).Should(HaveOccurred())
		})

		Describe("Bugs around the container lifecycle (#77768828)", func() {
			It("should support deleting a container after an errant delete", func() {
				handle := uniqueHandle()
				err := gardenClient.Destroy(handle)
				Ω(err).Should(HaveOccurred())

				_, err = gardenClient.Create(api.ContainerSpec{Handle: handle})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = gardenClient.Lookup(handle)
				Ω(err).ShouldNot(HaveOccurred())

				err = gardenClient.Destroy(handle)
				Ω(err).ShouldNot(HaveOccurred(), "Expected no error when attempting to destroy this container")

				_, err = gardenClient.Lookup(handle)
				Ω(err).Should(HaveOccurred())
			})

			It("should not allow creating an already existing container", func() {
				handle := uniqueHandle()

				_, err := gardenClient.Create(api.ContainerSpec{Handle: handle})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = gardenClient.Create(api.ContainerSpec{Handle: handle})
				Ω(err).Should(HaveOccurred(), "Expected an error when creating a Garden container with an existing handle")

				gardenClient.Destroy(handle)
			})
		})

		Describe("mounting docker images", func() {
			It("should mount an ubuntu docker image, just fine", func() {
				container, err := gardenClient.Create(api.ContainerSpec{
					Handle:     "my-ubuntu-based-docker-image",
					RootFSPath: "docker:///onsi/grace",
				})
				Ω(err).ShouldNot(HaveOccurred())

				process, err := container.Run(lsProcessSpec, silentProcessIO)
				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				gardenClient.Destroy("my-ubuntu-based-docker-image")
			})

			It("should mount a none-ubuntu docker image, just fine", func() {
				container, err := gardenClient.Create(api.ContainerSpec{
					Handle:     "my-none-ubuntu-based-docker-image",
					RootFSPath: "docker:///onsi/grace-busybox",
				})
				Ω(err).ShouldNot(HaveOccurred())

				process, err := container.Run(lsProcessSpec, silentProcessIO)
				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				gardenClient.Destroy("my-none-ubuntu-based-docker-image")
			})
		})

		Describe("BindMounts", func() {
			It("should mount a read-only BindMount (#75464648)", func() {
				runInVagrant("/usr/bin/sudo rm -f /var/bindmount-test")

				container, err := gardenClient.Create(api.ContainerSpec{
					Handle: "bindmount-container",
					BindMounts: []api.BindMount{
						api.BindMount{
							SrcPath: "/var",
							DstPath: "/home/vcap/my_var",
							Mode:    api.BindMountModeRO},
					},
				})
				Ω(err).ShouldNot(HaveOccurred())

				runInVagrant("sudo touch /var/bindmount-test")

				buffer := gbytes.NewBuffer()
				process, err := container.Run(api.ProcessSpec{
					Path: "ls",
					Args: []string{"-l", "/home/vcap/my_var"},
				}, recordedProcessIO(buffer))

				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				Ω(buffer.Contents()).Should(ContainSubstring("bindmount-test"))

				// Check mount really is read-only.
				buffer = gbytes.NewBuffer()
				process, err = container.Run(api.ProcessSpec{
					Path: "rm",
					Args: []string{"/home/vcap/my_var/bindmount-test"},
				}, recordedProcessIO(buffer))

				Ω(err).ShouldNot(HaveOccurred())
				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(1))
				Ω(buffer.Contents()).Should(ContainSubstring("Read-only file system"))

				runInVagrant("sudo rm -f /var/bindmount-test")
			})

			It("should mount a read/write BindMount (#75464648)", func() {
				container, err := gardenClient.Create(api.ContainerSpec{
					Handle: "bindmount-container",
					BindMounts: []api.BindMount{
						// Specify the container as origin, so SrcPath is a path
						// in the root file system rather than the host.
						api.BindMount{
							SrcPath: "/home/vcap",
							DstPath: "/home/vcap/vcaphome",
							Mode:    api.BindMountModeRW,
							Origin:  api.BindMountOriginContainer,
						},
					},
				})
				Ω(err).ShouldNot(HaveOccurred())

				// Can we write to it?
				buffer := gbytes.NewBuffer()
				process, err := container.Run(api.ProcessSpec{
					Path: "touch",
					Args: []string{"/home/vcap/vcaphome/bindmount-rw-test"},
				}, recordedProcessIO(buffer))

				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				buffer = gbytes.NewBuffer()
				process, err = container.Run(api.ProcessSpec{
					Path: "ls",
					Args: []string{"-l", "/home/vcap"},
				}, recordedProcessIO(buffer))

				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				Ω(buffer.Contents()).Should(ContainSubstring("bindmount-rw-test"))
			})
		})
	})

	// 	XDescribe("Bugs with snapshotting (#77767958)", func() {
	// 		BeforeEach(func() {
	// 			fmt.Println(`
	// !!!READ THIS!!!
	// Using this test is non-trivial.  You must:
	//
	// - Focus the "Bugs with snapshotting" Describe
	// - Make sure you are running bosh-lite
	// - Make sure the -snapshots flag is set in the control script for the warden running in your cell.
	// - Run this test the first time: this will create containers and both tests should pass.
	// - Run this test again: it should say that it will NOT create the container and still pass.
	// - bosh ssh to the cell and monit restart warden
	// - wait a bit and make sure warden is back up
	// - Run this test again -- this time these tests will fail with 500.
	// - Run it a few more times, eventually (I've found) it starts passing again.
	// `)
	// 		})
	//
	// 		It("should support snapshotting", func() {
	// 			handle := "snapshotable-container"
	// 			_, err := gardenClient.Lookup(handle)
	// 			if err != nil {
	// 				fmt.Println("CREATING CONTAINER")
	//
	// 				_, err = gardenClient.Create(api.ContainerSpec{
	// 					Handle: handle,
	// 					Env: []string{
	// 						"ROOT_ENV=A",
	// 						"OVERWRITTEN_ENV=B",
	// 						"HOME=/nowhere",
	// 					},
	// 				})
	// 				Ω(err).ShouldNot(HaveOccurred())
	// 			} else {
	// 				fmt.Println("NOT CREATING CONTAINER")
	// 			}
	//
	// 			container, err := gardenClient.Lookup(handle)
	// 			Ω(err).ShouldNot(HaveOccurred())
	// 			buffer := gbytes.NewBuffer()
	// 			process, err := container.Run(api.ProcessSpec{
	// 				Path: "bash",
	// 				Args: []string{"-c", "printenv"},
	// 				Env: []string{
	// 					"OVERWRITTEN_ENV=C",
	// 				},
	// 			}, recordedProcessIO(buffer))
	//
	// 			Ω(err).ShouldNot(HaveOccurred())
	//
	// 			process.Wait()
	//
	// 			Ω(buffer.Contents()).Should(ContainSubstring("OVERWRITTEN_ENV=C"))
	// 			Ω(buffer.Contents()).ShouldNot(ContainSubstring("OVERWRITTEN_ENV=B"))
	// 			Ω(buffer.Contents()).Should(ContainSubstring("HOME=/home/vcap"))
	// 			Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/nowhere"))
	// 			Ω(buffer.Contents()).Should(ContainSubstring("ROOT_ENV=A"))
	// 		})
	// 	})

})
