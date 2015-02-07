//usr/bin/env go run "$0" "$@" ; exit $?

package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
)

func failIf(err error, action string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, action, "failed:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	gardenClient := client.New(connection.New("tcp", "127.0.0.1:7777"))

	_ = gardenClient.Destroy("foo")
	container, err := gardenClient.Create(garden.ContainerSpec{
		Handle:     "foo",
		Env:        []string{"LANG=en_GB.iso885915"},
		RootFSPath: "docker:///debian#8",
	})
	failIf(err, "Create")

	var output bytes.Buffer
	process, err := container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{"-c", "echo $LANG"},
	}, garden.ProcessIO{Stdout: &output})
	failIf(err, "Run")
	process.Wait()
	fmt.Println(output.String())
}
