# 🚀 Release-Wave-action

![GitHub Action](https://img.shields.io/badge/GitHub-Action-blue?logo=github)

This is a custom GitHub action written in Go to automate release candidate branch creation.

## 📥 Inputs

| Name                | Description                                              | Default                     | Required |
|---------------------|----------------------------------------------------------|-----------------------------|----------|
| `rc_version`        | The version number of the release candidate.             | `1.0.0-rc`                  | true     |
| `owner`             | The owner of the repository.                             | `owner`                     | true     |
| `development_branch`| The development branch.                                  | `development`               | true     |
| `production_branch` | The default branch.                                      | `main`                      | true     |
| `pr_title`          | The title of the pull request.                           | `Release Candidate`         | true     |
| `pr_body`           | The body of the pull request.                            | `This is a release candidate` | true     |
| `github_token`      | The GitHub token.                                        |                             | false    |
| `app_id`            | The GitHub App ID.                                       |                             | false    |
| `private_key`       | The GitHub App private key.                              |                             | false    |
| `installation_id`   | The GitHub App installation ID.                          |                             | false    |

## 📤 Outputs

| Name          | Description                              |
|---------------|------------------------------------------|
| `pr_urls`     | The URLs of the created pull requests.   |
| `slack_payload`| The payload to be sent to Slack.        |