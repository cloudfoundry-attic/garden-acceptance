# Garden Acceptance Suite

## Running

1. Install Vagrant and VirtualBox
1. `vagrant up`
1. `vagrant ssh -c /vagrant/vagrant/run_tests`

## Updating Docker images

1. `docker login` and authenticate using the `cloudfoundry` account
1. `cd docker`
1. `./build_and_push`
