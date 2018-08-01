# drone-terraform-github-commenter

This plugin for Drone posts a comment to a GitHub PR, with the Terraform plan.

## Configuration

The following parameters are used to configure the plugin:

- `token`: The token to be used to create a GitHub comment. Required. Preferred to be a Drone secret.
- `title`: The title of the comment. Optional.
- `root_dir`: The root directory of where the Terraform plan ran. Default is `.`
- `filename`: The filename to use as the Terraform plan output. Default is `plan.out`.
- `type`: The type of comment. Possible values `pr`, `commit`. Default is `pr`

This plugin assumes you have Drone setup on an AWS EC2 instance, with an IAM instance profile.

### Drone configuration example

```yaml
pipeline:
  plan:
    image: jmccann/drone-terraform:5
    actions:
      - validate
      - plan

  comment-plan:
    image: robertstettner/drone-terraform-github-commenter
    token: 456deadbeef123
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
-   token: 456deadbeef123
+   secrets: [ github_token ]
```