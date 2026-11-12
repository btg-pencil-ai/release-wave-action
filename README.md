# 🚀 Release-Wave-action

![GitHub Action](https://img.shields.io/badge/GitHub-Action-blue?logo=github)

A Docker GitHub Action, written in Go, that runs a release wave across the repositories of a GitHub organization: it dispatches each repository's production release workflow, and can open sync pull requests from the production branch into epic branches.

## 🧭 Use cases

Set with the `use_case` input.

### `Production-Release`
1. Builds the repository list (see [Repository selection](#-repository-selection)).
2. For each repository, dispatches every workflow whose file name matches `prod-release.*` on `production_branch`, with the inputs `environment` and `release_version` (= `rc_version`).
3. Sets the `slack_payload` output.

If `hydra_webhook_url` is set, the start and the result of the wave are also reported to that webhook.

### `Main-To-Epic-Sync`
1. Gets the epic names: from `epics` if set, otherwise the active epic names from the webhook at `hydra_webhook_url`.
2. Finds the matching protected `epic-*` branches in each repository.
3. Closes old open sync pull requests for those epics and deletes their branches.
4. Creates `sync/<rc_version>-<epic>` from `production_branch` and opens a pull request into each matching epic branch, flagging merge conflicts.
5. Sets the `sync_pr_slack_payload` output.

## 📦 Repository selection
- If `include_repositories` is set, exactly those repositories are used and no exclusions apply.
- Otherwise every non-archived repository of `owner` is used, minus `exclude_repositories`. The same exclusions apply to both use cases.
- Lists are comma-separated.

## 📥 Inputs

| Name | Description | Default | Required |
|------|-------------|---------|----------|
| `use_case` | `Production-Release` or `Main-To-Epic-Sync` | | true |
| `rc_version` | Release version, `vX.Y.Z` (e.g. `v1.2.5`) | | true |
| `owner` | Organization / owner of the repositories | `owner` | true |
| `production_branch` | Branch to release from and to sync from | `main` | true |
| `environment` | Environment name passed to the production workflows | | false |
| `include_repositories` | Repositories to use instead of all of `owner`'s | | false |
| `exclude_repositories` | Repositories to skip | | false |
| `hydra_webhook_url` | Base URL of the status / active-epics webhook | | false¹ |
| `hydra_webhook_secret` | Secret used to sign webhook requests (HMAC-SHA256, `X-Hub-Signature-256` header) | | false |
| `epics` | For `Main-To-Epic-Sync`: comma-separated epic names to sync instead of the active ones | | false |
| `sync_trigger` | For `Main-To-Epic-Sync`: what started the sync, `release` (the sync after a production release) or `manual` | `release` | false |
| `triggered_by` | For `Main-To-Epic-Sync`: who started the sync | | false |
| `app_id` | GitHub App ID | | false² |
| `private_key` | GitHub App private key | | false² |
| `installation_id` | GitHub App installation ID | | false² |
| `github_token` | Token used when the GitHub App inputs are not set | | false² |

¹ Required for `Main-To-Epic-Sync` unless `epics` is set. Optional for `Production-Release`.
² Provide either the three GitHub App inputs or `github_token`.

## 📤 Outputs

| Name | Description |
|------|-------------|
| `slack_payload` | Slack Block Kit payload listing the repositories whose production workflows were dispatched |
| `sync_pr_slack_payload` | Slack Block Kit payload listing the sync pull requests per epic, with links, conflicts and errors |

## 🚀 Usage

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Release wave
        id: release
        uses: <owner>/release-wave-action@main
        with:
          use_case: "Production-Release"
          rc_version: "v1.2.5"
          environment: "production"
          owner: "<owner>"
          production_branch: "main"
          app_id: ${{ secrets.APP_ID }}
          private_key: ${{ secrets.APP_PRIVATE_KEY }}
          installation_id: ${{ secrets.APP_INSTALLATION_ID }}
          exclude_repositories: "repo-a,repo-b"

      - name: Slack notification
        uses: slackapi/slack-github-action@v2.0.0
        with:
          payload: ${{ steps.release.outputs.slack_payload }}
          webhook-type: incoming-webhook
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}

  sync:
    needs: release
    runs-on: ubuntu-latest
    steps:
      - name: Sync pull requests
        id: sync
        uses: <owner>/release-wave-action@main
        with:
          use_case: "Main-To-Epic-Sync"
          rc_version: "v1.2.5"
          owner: "<owner>"
          production_branch: "main"
          app_id: ${{ secrets.APP_ID }}
          private_key: ${{ secrets.APP_PRIVATE_KEY }}
          installation_id: ${{ secrets.APP_INSTALLATION_ID }}
          exclude_repositories: "repo-a,repo-b"
          hydra_webhook_url: ${{ vars.WEBHOOK_URL }}
          hydra_webhook_secret: ${{ secrets.WEBHOOK_SECRET }}

      - name: Slack notification
        if: ${{ always() && steps.sync.outputs.sync_pr_slack_payload != '' }}
        uses: slackapi/slack-github-action@v2.0.0
        with:
          payload: ${{ steps.sync.outputs.sync_pr_slack_payload }}
          webhook-type: incoming-webhook
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

## 🛠️ Development

```bash
go build ./...
go vet ./...
```
