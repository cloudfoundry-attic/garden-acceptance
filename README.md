# Garden Acceptance Suite

## Running

1. Install Vagrant and VirtualBox
1. `vagrant up`
1. `./scripts/test`

## Updating garden-linux

Run `sudo /vagrant/vagrant/install_garden_linux <SHA>` on the vagrant machine.

## Updating Docker images

1. `docker login` and authenticate using the `cloudfoundry` account
1. `cd docker`
1. `./build_and_push`
