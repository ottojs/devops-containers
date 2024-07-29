#!/usr/bin/env bash

# Debian 12
# Run as root
#
# Edit this
export YOUR_IP="127.0.0.1";

# Upgrade System Packages
apt-get update;
apt-get dist-upgrade -y;
apt-get autoremove;

# Install HAProxy
# https://haproxy.debian.net/#distribution=Debian&release=bookworm&version=3.0
curl https://haproxy.debian.net/bernat.debian.org.gpg \
      | gpg --dearmor > /usr/share/keyrings/haproxy.debian.net.gpg;
echo deb "[signed-by=/usr/share/keyrings/haproxy.debian.net.gpg]" \
      https://haproxy.debian.net bookworm-backports-3.0 main \
      > /etc/apt/sources.list.d/haproxy.list;
apt-get update;
apt-get install haproxy=3.0.\*;

# Install Podman
apt-get install -y podman;
sed -i "s/# unqualified-search-registries = \[\"example.com\"\].*/unqualified-search-registries = \[\"docker.io\"\]/" /etc/containers/registries.conf;

# Install Python
apt-get install -y python3 python3-pip pipx python-is-python3;

# Install Podman Compose
pipx install podman-compose;
echo 'export PATH="~/.local/bin:${PATH}"' >> ~/.profile;

# Install Firewall
apt-get install -y ufw;
sed -i "s/IPV6=.*/IPV6=no/" /etc/default/ufw;
ufw allow 80;
ufw allow 443;
ufw allow from "${YOUR_IP}" to any port 22;
ufw allow from "${YOUR_IP}" to any port 8443;
ufw allow in on podman1 to any;
ufw enable;
