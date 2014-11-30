# vim: set ft=ruby

Vagrant.configure("2") do |config|
  config.vm.hostname = "garden"
  config.vm.box = "ubuntu/trusty64"

  config.vm.network "private_network", ip: "192.168.50.5"
  config.vm.network "forwarded_port", guest: 7777, host: 7777

  config.vm.provider :virtualbox do |vb, override|
    vb.gui = false
    vb.name = "garden"
    vb.cpus = 4
    vb.memory = 8 * 1024
  end

  go_url = "https://storage.googleapis.com/golang/go1.3.1.linux-amd64.tar.gz"
  garden_linux_repo = "https://github.com/cloudfoundry-incubator/garden-linux"

	config.vm.provision "shell", inline: <<-EOS
    set -e
    apt-get update
    apt-get install -y git mercurial
    curl -s -o /tmp/go.tgz #{go_url}
    tar -C /usr/local -xzf /tmp/go.tgz
    rm /tmp/go.tgz
    sudo mkdir -p /opt/garden/{containers,snapshots,overlays,rootfs}
		curl -s -o lucid64.dev.tgz http://cf-runtime-stacks.s3.amazonaws.com/lucid64.dev.tgz
		sudo tar -xzpf lucid64.dev.tgz -C /opt/garden/rootfs
    export GOPATH=/home/vagrant/go
    export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
    mkdir $GOPATH
    cd $GOPATH
    go get github.com/tools/godep
    git clone #{garden_linux_repo} $GOPATH/src/github.com/cloudfoundry-incubator/garden-linux
    cd $GOPATH/src/github.com/cloudfoundry-incubator/garden-linux
    echo 'Godep Restoring...'
    godep restore
    echo 'Running make...'
    make
    echo 'Building garden-linux...'
    go build -a -tags daemon -o out/garden-linux
  EOS
end
