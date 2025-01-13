# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  
  config.vm.box = "ubuntu/jammy64"

  config.vm.network "public_network"

  config.vm.provider "virtualbox" do |vb|
    vb.gui = true
    vb.name = "vault_plugin_dev"
    vb.memory = "16384"
    vb.cpus = 6
  end

  config.vm.provision "shell" do |s|
    s.path = "setup_as_root.sh"
    s.privileged = true
  end
  config.vm.provision "shell" do |s|
    s.path = "setup_as_user.sh"
    s.privileged = false
  end

end
