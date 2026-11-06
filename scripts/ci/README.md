# AskXuan CI release tooling

`release.py` builds Linux amd64 binaries or web distributions on GitHub and packages immutable files, checksums, exact Git revisions and ancestors. `publish.sh` uploads to the pinned ECS SSH host using a component-specific forced command.

`ecs_receiver.py` is installed separately as root-owned `/usr/local/sbin/askxuan-ci-receiver`. It validates archives before changing runtime state, serializes all repositories using flock, preserves other components, checks runtime health and rolls back failures. It never executes scripts from the archive. SQL and runtime-template changes require an operator-reviewed contract baseline before deployment.

The `web` scope contains exactly two components: `admin` (including all commerce business pages) and `temple`. The admin build must emit the self-contained `legacy/shop/index.html` compatibility entry. The receiver replaces the obsolete standalone `/shop` assets with this entry in the same atomic static release, verifies old nested URLs, and records `web/shop` as an alias owned by `web/admin`, not a release component. H5-only deployments preserve both management applications and this alias. Failed checks restore the previous public pointer and complete provenance state.

When adopting this contract, first test and commit the tooling, install the reviewed receiver under the existing publication lock, and verify its checksum. Then pin frontend workflows to that tooling commit and publish the two-component archive. The receiver rejects obsolete three-component Web archives. Keep the immediately previous receiver and active runtime rollback releases available for operator recovery.

Run receiver tests: `python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v`.

Quiet park is an independent `quiet` scope. `GOWORK=off python3 scripts/ci/release.py quiet`
builds the entire `services/content/quiet-service` package for Linux amd64 and includes
exactly `payload/quiet/quiet`. It records `backend/quiet`; the original twenty-service
`backend` scope, contract, and input hashes do not include quiet. Quiet source must not
be added to the shared `go.work` for release builds. The independent
`quiet-release.yml` verifies race/protocol tests, real MySQL/Redis persistence, vet and
release regressions before uploading `quiet-release`. It does not deploy automatically.

An operator installs the reviewed receiver and separately provisions the root-owned
`/opt/askxuan/ci/runtime/quiet.json` template and `askxuan/quiet-service:ci-runtime-base`
image before invoking `askxuan-ci-receiver quiet < verified-quiet-release.tgz` as root
or through a dedicated scope-specific forced SSH command. Do not reuse a key forced
to the twenty-service `backend` command: that command deliberately rejects quiet.
Keep the existing global publication lock in place for receiver/template installation.

For the first installation, a reviewed root-owned copy of
`scripts/ops/bootstrap-quiet-runtime.py --base-image alpine@sha256:<reviewed-local-digest>`
prints a read-only plan by default. Add `--apply` only after reviewing that plan.
The script uses the same existing publish lock, rejects unfinished transactions
and any existing quiet database/user/container/volume/template/base tag, and
requires 3 GiB free. It reads the MySQL root password only from the server-local, root-owned mode0600
`/opt/askxuan/runtime/secrets.env`. Only the MySQL subprocess receives `MYSQL_PWD`;
no password enters argv, SQL stdin, build commands or receipts. SQL goes over stdin. It creates a new
`quiet_local` database and random-password `quiet` user with only CREATE, SELECT,
INSERT, UPDATE and DELETE on `quiet_local.*`; no other schema is changed.

It reuses the existing Redis container's immutable image ID to create a dedicated
`askxuan-quiet-redis` container and `askxuan-quiet-redis-data` volume with a new
random password, `askxuan-net`, and no host ports. It prepares only the runtime
image layer from the explicitly supplied local Alpine digest, with `/app` and
uid 1000, then verifies CA certificates/wget/user/directory before creating storage.
No application source is compiled. Runtime secrets, a template copy and a
stage receipt are retained under root-only `backups/quiet-bootstrap-*/`.
Any failure stops immediately and preserves all new data/resources and the
recovery receipt; there is no automatic cleanup or resume. An operator must
review a partial installation before any further action. The receiver manages
only the app, never this Redis container or either data store.

The template is Compose JSON with only `quiet-service`, container name
`askxuan-quiet-service`, the fixed base image, `user: "1000:1000"`,
`entrypoint: ["/app/quiet"]`, no volumes, and a healthcheck against
`http://127.0.0.1:8096/health`. Its one port is
`{"target":8096,"published":"18196","host_ip":"127.0.0.1","protocol":"tcp"}`.
Its network is `askxuan-net` declared external with that exact name. The base image
must already provide `/app`, CA certificates, and the healthcheck executable (wget).
Receiver builds contain only a generated `FROM`/`COPY` Dockerfile, never archive scripts.

Runtime environment keys are `QUIET_LISTEN=0.0.0.0:8096`, `QUIET_MYSQL_ADDR`,
`QUIET_MYSQL_PASSWORD`, `QUIET_REDIS_ADDR`, `QUIET_REDIS_PASSWORD`,
`QUIET_CSRF_KEY`, `QUIET_REDIS_PREFIX=quiet-park-prod:`, and explicit HTTPS
`QUIET_PUBLIC_ORIGINS`. `QUIET_AUTH_URL` is optional. The module uses database
`quiet_local` and MySQL user `quiet`; storage/password provisioning remains an operator
action. All template parents and the file must be root-owned, non-symlink and not
group/world writable. The template and generated runtime files can contain secrets;
keep them root-only and never include them in a release archive or CI artifact.

The receiver manages only the application container, reuses the global lock, archive
checks, ancestor validation and 3 GiB free-space threshold, and checks persistent
health on loopback port 18196. On first-deploy failure or rollback, it removes only
`askxuan-quiet-service`, without `-v`; MySQL/Redis and their data remain. Later releases
preserve existing runtime configuration and healthcheck and retain the old image for
rollback. Operator rollback uses the existing `--rollback <release-id>` command and
only accepts the latest global completed release.

If public artifact storage alone fails, the private H5 `private-source-relay.yml`
supports `quiet`. Its fail-closed gate requires the exact reviewed backend SHA,
the pinned quiet workflow bytes, all five quiet test/build steps, and a failure only
at the known artifact upload step. The source allowlist starts disabled until a
reviewed commit is pinned. The private runner rebuilds only quiet without credentials
and retains a private draft package with provenance; it does not deploy and does not
claim that rebuilt bytes equal the public runner's original archive.

Run workflows from main/master, or manually using workflow_dispatch. Only protected production deployment jobs receive secrets. Each frontend workflow pins this repository's tooling commit rather than executing a floating release script.

Operational guide: the documentation repository's `docs/deployment/GITHUB-ACTIONS.md`.

园区公网代理单独使用 `scripts/ops/refresh-quiet-proxy.sh deploy/nginx <new-root-backup-dir>` 安装；同一发布锁下备份三份文件，正常失败与HUP/INT/TERM均恢复，`nginx -t`通过才reload。`quiet-limits.conf`位于http层，`quiet-locations.conf`仅追加到既有应用server include；固定quiet-park预算键跨IP域名和站点别名共享，健康Ping仅loopback可见。先确认CI quiet容器健康，再启用这些路由，最后发布H5入口。
