
## Configuration

The application reads its configuration from `config.yaml`. The configuration file supports the following fields:

- `interval`: Interval in seconds for the application to run its tasks. Default is `5`.
- `resolvConf`: Path to the `resolv.conf` file. Default is `/systemd-resolve/resolv.conf`.
- `corednsConfDir`: Directory for CoreDNS configuration files. Default is `/coredns/conf/`.
- `corednsCorefile`: Path to the CoreDNS `Corefile`. Default is `/etc/coredns/Corefile`.

## Usage

### Build image:

```sh
docker buildx build --platform linux/amd64 --tag coredns-reload:v1.0 . --load
```

### Export image:

```sh
docker save coredns-reload -o image.tar
```

### Deploy:

1. As k3s manifest:
  - add `--disable=coredns` arg to k3s service and restart it;
  - if a cluster DNS IP is used, udpate the service `clusterIP` on `deploy/k3s-manifest.yaml`
  - copy the `deploy/k3s-manifest.yaml` to `/var/lib/rancher/k3s/server/manifests/`
  