# Registry production operations

This runbook covers one hardened Machine Genome registry node. It is an
operational service, not by itself a neutral standards authority or witness
network.

## Security boundary

- Terminate TLS in the process or at a trusted same-host proxy. The CLI refuses
  cleartext non-loopback listeners unless the operator explicitly overrides it.
- Keep the checkpoint seed and administrator token outside the repository.
  Both files must deny group/other access. Prefer encrypted systemd credentials.
- Keep the data directory private. The service enforces directory mode `0700`
  and database mode `0600`.
- Do not place personal data, secrets, prompts, mailbox contents, or mutable
  evidence at public registry URLs. Publish commitments.
- Rate limiting is per direct peer address. If a reverse proxy is used, enforce
  client limits there too; this service deliberately does not trust forwarded
  address headers.

## systemd example

Provision `/usr/local/bin/mgs`, create a dedicated `machine-genome` system user,
and install encrypted credentials named `checkpoint-key` and `admin-token`.

```ini
[Unit]
Description=Machine Genome Registry
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=machine-genome
Group=machine-genome
StateDirectory=machine-genome
LoadCredentialEncrypted=checkpoint-key
LoadCredentialEncrypted=admin-token
ExecStart=/usr/local/bin/mgs registry-serve \
  --data-dir=/var/lib/machine-genome \
  --checkpoint-key=%d/checkpoint-key \
  --admin-token-file=%d/admin-token \
  --listen=127.0.0.1:8080 \
  --public-base-url=https://registry.example
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
LockPersonality=yes
RestrictSUIDSGID=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
```

Use `--tls-cert` and `--tls-key` when the process terminates TLS itself. The
server sets read-header, read, write, idle, header-size, and graceful-shutdown
limits. `SIGTERM` allows up to ten seconds for in-flight requests.

The production reverse-proxy examples are
`deploy/machinegenome.org.nginx.conf` and `deploy/cloudflare-real-ip.conf`.
The nginx layer terminates TLS, redirects `www` to the apex origin, applies a
per-client request limit, and only accepts `CF-Connecting-IP` from Cloudflare's
published networks. Refresh that network list before deployment. Because nginx
owns the public client limit and the Go service is loopback-only, the packaged
systemd unit uses a high backend limit to avoid pooling all proxied clients into
the loopback address.

## Health and monitoring

- Liveness: `GET /healthz/live`
- Readiness: `GET /healthz/ready`
- Metrics: `GET /metrics`
- Current signed root: `GET /v1/log/checkpoint`

Alert on readiness failures, restart loops, growing rejected-request counts,
unexpected tree-size stalls, disk pressure, and a checkpoint root that differs
between independent observers at the same tree size.

## Backup and restore

Create a transactionally consistent, mode-`0600` online backup through the
authenticated service boundary:

```sh
curl -fsS -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://registry.example/v1/admin/backups
```

The default destination is `DATA_DIR/backups`; use `--backup-dir` to place it
on a separate private filesystem. `mgs registry-backup` remains available for
offline maintenance when the service is stopped.

Copy backups to an encrypted off-host destination and test restoration. To
restore, stop the service, preserve the failed database for forensics, place a
verified backup at `DATA_DIR/registry.db` with mode `0600`, and run:

```sh
mgs registry-verify --data-dir DATA_DIR
```

The integrity command checks content digests, metadata/object correspondence,
and every link in the administrative audit hash chain. After restart, compare a
fresh signed checkpoint with retained observer checkpoints. Never silently
replace a newer public tree with an older backup.

## Key and incident handling

Checkpoint key rotation and witness ceremonies are not yet standardized by this
profile. Treat compromise as an incident: stop signing, preserve logs and data,
publish the last trusted checkpoint out of band, and disclose the replacement
key with an explicit continuity statement. Rotating only the admin token does
not alter public log identity.

For illegal or dangerous payload availability, create a reasoned tombstone
instead of deleting database history. Record appeal and case identifiers in the
external case system; do not put confidential case content in the public API.
