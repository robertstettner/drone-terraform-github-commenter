# drone-terraform-github-commenter

This plugin for Drone posts a comment to a GitHub PR, with the Terraform plan output.

## Configuration

The following parameters are used to configure the plugin:

- `title`: The title of the comment. Default is `Terraform Plan Output`.
- `recreate`: A flag to recreate the comment every time, otherwise comment is updated based on the title. Default is `false`.
- `issue_num`: The PR or Issue number to post the comment. Optional.
- `root_dir`: The root directory of where the Terraform plan ran. Default is `.`
- `tf_data_dir`: The data directory where Terraform stores providers, plugins, and modules. Default is `.terraform`.
- `tf_version`: The Terraform version to download and use, when not provided uses the prepackaged Terraform in the Docker image. Optional.
- `base_url`: The GitHub Base URL, for use with GitHub Enterprise Server. Default is `https://api.github.com/`.

### Secrets

All the following secrets are optional:

- `github_username`
- `github_password`
- `github_token` or `github_release_api_key`

This plugin is setup to use the GitHub credentials from Drone's netrc environment variables.

### Drone configuration example

```yaml
pipeline:
  plan:
    image: jmccann/drone-terraform:6.3-0.12.20
    actions:
      - validate
      - plan

  comment-plan:
    image: robertstettner/drone-terraform-github-commenter
```

Usage with Drone secret:

```diff
pipeline:
  plan:
    image: jmccann/drone-terraform:5
    actions:
      - validate
      - plan

  comment-plan:
    image: robertstettner/drone-terraform-github-commenter
+   secrets: [ github_token ]
```
