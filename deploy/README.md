# Deployment assets

The reference production topology terminates TLS at nginx and keeps the Go
registry on loopback. These assets are examples that must be reviewed against
the target host and secret-management policy.

## Files

| File | Purpose |
| --- | --- |
| `machine-genome-registry.service` | Hardened systemd service using credential mounts |
| `machinegenome.org.nginx.conf` | HTTPS, canonical redirect, proxy limits, and security headers |
| `cloudflare-real-ip.conf` | Restrict `CF-Connecting-IP` trust to Cloudflare networks |
| `container-compose.example.yaml` | Local container composition behind a loopback port |

## Native systemd deployment

1. Build a static binary with `make build`.
2. Create a locked service account and private directories.
3. Provision a checkpoint key and administrator token outside the repository.
4. Install the binary and unit.
5. Install the nginx files, obtain a certificate, validate with `nginx -t`, and
   reload.
6. Start the registry and run the acceptance checks in
   [registry-operations.md](../docs/registry-operations.md).

Example provisioning outline:

```sh
sudo useradd --system --home /var/lib/machine-genome \
  --shell /usr/sbin/nologin machine-genome
sudo install -d -o machine-genome -g machine-genome -m 0700 \
  /var/lib/machine-genome /var/lib/machine-genome/backups
sudo install -d -o root -g root -m 0700 /etc/machine-genome
sudo install -o root -g root -m 0755 bin/mgs /usr/local/bin/mgs
sudo install -o root -g root -m 0644 \
  deploy/machine-genome-registry.service \
  /etc/systemd/system/machine-genome-registry.service
sudo systemctl daemon-reload
sudo systemctl enable --now machine-genome-registry
```

Do not paste private key or token values into shell history. Production
operators should use encrypted systemd credentials or an equivalent secret
manager.

## Container image

The root `Dockerfile` produces a static, distroless, non-root image:

```sh
docker build \
  --build-arg VERSION=0.1.1 \
  --build-arg COMMIT="$(git rev-parse HEAD)" \
  -t machine-genome:0.1.1 .
docker run --rm machine-genome:0.1.1 version
```

The compose example binds the registry only to host loopback. Pre-create its
bind-mounted data directory for the distroless non-root UID:

```sh
install -d -m 0700 .local/registry-data
sudo chown 65532:65532 .local/registry-data
cp deploy/container-compose.example.yaml compose.yaml
CHECKPOINT_KEY_FILE=/absolute/path/checkpoint.key.json \
ADMIN_TOKEN_FILE=/absolute/path/admin.token \
docker compose up --build
```

The container flag `--allow-insecure-public` permits HTTP on the container
interface only because Docker publishes it to host loopback for a trusted
same-host TLS proxy. Never publish that port directly to an untrusted network.

## Cloudflare

`cloudflare-real-ip.conf` must match Cloudflare's current published ranges:
<https://www.cloudflare.com/ips/>. The nginx real-IP module accepts the
`CF-Connecting-IP` header only from those ranges. If DNS records are not
proxied, nginx continues to use the direct peer address.

## Upgrade

Before replacing a binary:

1. retain the current signed checkpoint;
2. create and verify an online backup;
3. run repository and release gates;
4. install the new binary atomically;
5. restart and verify PID/executable identity, readiness, log size/root,
   inclusion proof, security headers, and explorer views;
6. retain the previous artifact until the observation window closes.

Never silently restore an older database over a newer public transparency tree.
