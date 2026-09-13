# AskXuan CI release tooling

`release.py` builds Linux amd64 binaries or web distributions on GitHub and packages immutable files, checksums, exact Git revisions and ancestors. `publish.sh` uploads to the pinned ECS SSH host using a component-specific forced command.

`ecs_receiver.py` is installed separately as root-owned `/usr/local/sbin/askxuan-ci-receiver`. It validates archives before changing runtime state, serializes all repositories using flock, preserves other components, checks runtime health and rolls back failures. It never executes scripts from the archive. SQL and runtime-template changes require an operator-reviewed contract baseline before deployment.

The `web` scope contains exactly two components: `admin` (including all commerce business pages) and `temple`. The admin build must emit the self-contained `legacy/shop/index.html` compatibility entry. The receiver replaces the obsolete standalone `/shop` assets with this entry in the same atomic static release, verifies old nested URLs, and records `web/shop` as an alias owned by `web/admin`, not a release component. H5-only deployments preserve both management applications and this alias. Failed checks restore the previous public pointer and complete provenance state.

When adopting this contract, first test and commit the tooling, install the reviewed receiver under the existing publication lock, and verify its checksum. Then pin frontend workflows to that tooling commit and publish the two-component archive. The receiver rejects obsolete three-component Web archives. Keep the immediately previous receiver and active runtime rollback releases available for operator recovery.

Run receiver tests: `python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v`.

Run workflows from main/master, or manually using workflow_dispatch. Only protected production deployment jobs receive secrets. Each frontend workflow pins this repository's tooling commit rather than executing a floating release script.

Operational guide: the documentation repository's `docs/deployment/GITHUB-ACTIONS.md`.
