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
    vb.memory = 4 * 1024
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
  end

  go_url = "https://storage.googleapis.com/golang/go1.3.1.linux-amd64.tar.gz"
  garden_linux_repo = "https://github.com/cloudfoundry-incubator/garden-linux"

  config.vm.provision "shell", inline: <<-EOS
    set -e
    echo ":::: Improving SSH.."
    echo "UseDNS no" >> /etc/ssh/sshd_config
    echo "GSSAPIAuthentication no" >> /etc/ssh/sshd_config

    echo ":::: Updating..."
    apt-get update -qq -y > /dev/null
    apt-get install -qq -y git mercurial > /dev/null

    echo ":::: Installing Go..."
    curl -s -o /tmp/go.tgz #{go_url}
    tar -C /usr/local -xzf /tmp/go.tgz
    rm /tmp/go.tgz

    echo ":::: Preparing directories..."
    sudo mkdir -p /opt/garden/{containers,snapshots,overlays,rootfs}
    curl -s -o lucid64.dev.tgz http://cf-runtime-stacks.s3.amazonaws.com/lucid64.dev.tgz
    sudo tar -xzpf lucid64.dev.tgz -C /opt/garden/rootfs

    echo ":::: Installing godep..."
    export GOPATH=/home/vagrant/go
    export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
    mkdir $GOPATH
    cd $GOPATH
    go get github.com/tools/godep

    echo ":::: Installing garden-linux..."
    git clone #{garden_linux_repo} $GOPATH/src/github.com/cloudfoundry-incubator/garden-linux
    cd $GOPATH/src/github.com/cloudfoundry-incubator/garden-linux
    echo 'Godep Restoring...'
    # https://www.pivotaltracker.com/n/projects/1158420/stories/89281608
    godep restore || godep restore
    echo 'Running make...'
    make
    echo 'Building garden-linux...'
    go build -a -tags daemon -o out/garden-linux
  EOS
end
