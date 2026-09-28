# Integration tests

These tests require a real Docker daemon and are excluded from the default
`go test ./...` run via the `integration` build tag (PRD section 19:
Integration testing, Failure testing, Recovery testing, Backup/restore
testing).

## Running

```bash
cp .env.example .env
$EDITOR .env   # set real secrets — required, or the tests skip themselves

go test -tags integration ./tests/integration/... -v
```

Each test deploys the real stack (`docker/docker-compose.yml`) against a
temporary backup directory and tears it down afterward
(`t.Cleanup(client.ComposeDown)`). Run them on the target VM, not inside a
sandbox without a Docker daemon.

| Test | Exercises |
|---|---|
| `TestFullDeployLifecycle` | doctor → deploy → health (Core User Journey) |
| `TestFailureAndRecovery` | stop a container → health detects it → restart → recovers |
| `TestBackupAndRestore` | backup → validate manifest → restore |
