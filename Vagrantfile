# vim: set ft=ruby

ENV['VAGRANT_DEFAULT_PROVIDER'] = 'virtualbox'

Vagrant.configure("2") do |config|
  config.vm.hostname = "garden"
  config.vm.box = "ubuntu/trusty64"

  config.vm.network "private_network", ip: "192.168.50.5"
  config.vm.network "forwarded_port", guest: 7777, host: 7777
  config.vm.network "forwarded_port", guest: 7776, host: 7776

  config.vm.provider :virtualbox do |vb, override|
    vb.gui = false
    vb.name = "garden"
    vb.cpus = 4
    vb.memory = 4 * 1024
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
  end

  config.vm.provision "shell", path: "vagrant/provision", args: "a51aa9343950796b9251f8f1f64dc2e3052b1c32"
end
