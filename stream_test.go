package garden_acceptance_test

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = PDescribe("streaming in/out of containers", func() {
	Describe("StreamIn", func() {
		var tarStream io.Reader

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir("", "some-temp-dir-parent")
			Ω(err).ShouldNot(HaveOccurred())
			tarPath := filepath.Join(tmpdir, "some.tar")

			tarFile, err := os.Create(tarPath)
			Ω(err).ShouldNot(HaveOccurred())

			body := "i-am-a-file-and-i-got-streamed-in\n"
			size := int64(len(body))
			header := &tar.Header{
				Name: "the_file",
				Mode: 0777,
				Size: size,
			}

			writer := tar.NewWriter(tarFile)
			err = writer.WriteHeader(header)
			Ω(err).ShouldNot(HaveOccurred())
			bytesWritten, err := writer.Write([]byte(body))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(bytesWritten).Should(Equal(int(size)))
			err = writer.Close()
			Ω(err).ShouldNot(HaveOccurred())

			err = tarFile.Close()
			Ω(err).ShouldNot(HaveOccurred())

			// tarFile, err := os.Open(tarPath)
			// Expect(err).ToNot(HaveOccurred())

			// tarStream = tar.NewReader(tarFile)
			fmt.Println(tarPath)
			time.Sleep(time.Second * 60)
		})

		It("works", func() {
			container := createContainer(gardenClient, garden.ContainerSpec{})

			err := container.StreamIn(garden.StreamInSpec{
				Path:      "/some.tar",
				TarStream: tarStream,
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := gbytes.NewBuffer()
			process, err := container.Run(
				garden.ProcessSpec{User: "root", Path: "cat", Args: []string{"/new_file"}},
				recordedProcessIO(buffer),
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(process.Wait()).Should(Equal(0))
			Ω(buffer).Should(gbytes.Say("i got streamed in"))
		})
	})
})
