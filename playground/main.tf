module "issue-opener" {
  source = "../github-issue-opener/iac"

  # name is used to prefix resources created by this demo application
  # where possible.
  name = "chainguard-dev"

  # This is the GCP project ID in which certain resource will live including:
  #  - The container image for this application,
  #  - The Cloud Run service hosting this application,
  #  - The Secret Manager secret holding the github access token
  #    for opening issues.
  project_id = var.gcp_project_id

  # The Chainguard IAM group from which we expect to receive events.
  # This is used to authenticate that the Chainguard events are intended
  # for you, and not another user.
  group = var.chainguard_iam_group

  env = "enforce.dev"

  # These describe the github organization and repository in which github issues
  # will be opened.
  github_org  = "imjasonh"
  github_repo = "event-test"

  # These are the labels that get applied to opened issues.
  labels = "label1,label2,label3"
}

output "secret-command" {
  value = module.issue-opener.secret-command
}

output "url" {
  value = module.issue-opener.url
}
