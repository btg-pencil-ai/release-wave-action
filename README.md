# 🚀 Release-Wave-action

![GitHub Action](https://img.shields.io/badge/GitHub-Action-blue?logo=github)

This is a custom GitHub action written in Go to automate release candidate branch creation.

## 📥 Inputs

- `rc_version`: The version number of the release candidate. Default: `1.0.0-rc`. **Required: true**.
- `owner`: The owner of the repository. Default: `owner`. **Required: true**.
- `development_branch`: The development branch. Default: `development`. **Required: true**.
- `production_branch`: The default branch. Default: `main`. **Required: true**.
- `pr_title`: The title of the pull request. Default: `Release Candidate`. **Required: true**.
- `pr_body`: The body of the pull request. Default: `This is a release candidate`. **Required: true**.
- `github_token`: The GitHub token. **Required: false**.
- `app_id`: The GitHub App ID. **Required: false**.
- `private_key`: The GitHub App private key. **Required: false**.
- `installation_id`: The GitHub App installation ID. **Required: false**.

## 📤 Outputs

- `pr_urls`: The URLs of the created pull requests.
- `slack_payload`: The payload to be sent to Slack.