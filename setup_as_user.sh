
systemctl --user start dbus
dockerd-rootless-setuptool.sh install
docker context use rootless

mkdir -p /home/vagrant/.config/docker/
tee /home/vagrant/.config/docker/daemon.json <<EOF
{
  "runtimes": {
    "runsc": {
      "path": "/usr/bin/runsc",
      "runtimeArgs": [
        "--host-uds=create",
        "--ignore-cgroups"
      ]
    }
  }
}
EOF
systemctl --user restart docker
echo 'export DOCKER_HOST=unix:///run/user/1000/docker.sock' >> /home/vagrant/.bashrc


mkdir -p "$HOME/bin"
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz -O go.tar.gz
rm -rf "$HOME/bin/go" && tar -C "$HOME/bin" -xzf go.tar.gz
echo '[ "${PATH#*$HOME/bin/go/bin}" == "$PATH" ] && export PATH="$PATH:$HOME/bin/go/bin"' >> "${HOME}/.bashrc"

