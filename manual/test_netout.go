//usr/bin/env go run "$0" "$@" ; exit $?

package main

import (
	"fmt"
	"net"
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

func restartGarden() {
	_, _ = runInVagrant("sudo /var/vcap/bosh/bin/monit restart garden")
	time.Sleep(15 * time.Second)
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

func main() {
	gardenClient := client.New(connection.New("tcp", "127.0.0.1:7777"))

	_ = gardenClient.Destroy("foo")
	foo, err := gardenClient.Create(garden.ContainerSpec{Handle: "foo"})
	failIf(err, "Create")

	err = foo.NetOut(garden.NetOutRule{
		Protocol: garden.ProtocolICMP,
		Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.8.8.8"))},
	})
	failIf(err, "NetOut")

	restartGarden()

}
