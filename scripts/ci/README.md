# AskXuan CI release tooling

`release.py` builds Linux amd64 binaries or web distributions on GitHub and packages immutable files, checksums, exact Git revisions and ancestors. `publish.sh` uploads to the pinned ECS SSH host using a component-specific forced command.

`ecs_receiver.py` is installed separately as root-owned `/usr/local/sbin/askxuan-ci-receiver`. It validates archives before changing runtime state, serializes all repositories using flock, preserves other components, checks runtime health and rolls back failures. It never executes scripts from the archive. SQL and runtime-template changes require an operator-reviewed contract baseline before deployment.

Run receiver tests: `python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v`.

Run workflows from main/master, or manually using workflow_dispatch. Only protected production deployment jobs receive secrets. Each frontend workflow pins this repository's tooling commit rather than executing a floating release script.

Operational guide: the documentation repository's `docs/deployment/GITHUB-ACTIONS.md`.
