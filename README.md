# Otto.js DevOps - SelfHost

## Requirements

- Debian 12
- Root access

## Usage

1. Edit and run `install.sh` on remote machine
1. Upload certificate files in combo form `cert chain + key` in one file to `/etc/ssl/private/`
1. Run on remote machine `find /etc/ssl/private/ -name *combo > /etc/ssl/private/list.txt` to generate certificate list
1. Locally, edit `haproxy-generator/main.go` with your services
1. Run `go run haproxy-generator/main.go` to generate 3 files. `haproxy.cfg`, `docker-compose.yml`, and `update.sh`
1. Upload `haproxy.cfg` to `/etc/haproxy/haproxy.cfg`
1. Upload `docker-compose.yml` and `update.sh` to home directory (`/root`)
1. Upload `docker-compose-systemd.service` to `/etc/systemd/system/docker-compose-systemd.service`
1. Run `systemctl daemon-reload` to pick up the new file `/etc/systemd/system/docker-compose-systemd.service`
1. Run `systemctl enable haproxy` to start it at boot.
1. Run `systemctl enable docker-compose-systemd` to start podman compose at boot.
1. Build and save your service container images to tar files (see below)
1. Upload your service name tar files to home directory (`/root`)
1. Run `./update.sh` to import the container and restart services (see that script for details)

## Building Service Containers

```bash
# Build
podman build -t myservice:latest .;
# Export
podman save -o myservice.tar localhost/myservice:latest;
# Upload
scp myservice.tar root@[IP_OR_HOST_HERE]:;
# Import
ssh root@[IP_OR_HOST_HERE] "./update.sh";
```
