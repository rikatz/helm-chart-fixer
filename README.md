# Helm Chart fixed

This lazy program will:
* get the index.yaml of a helm chart branch from a project
* get the artifact for this chart
* check if there is any "k8s.gcr.io" content on values.yaml
* mutate to registry.k8s.io
* compress again
* get digest
* TODO: push new artifact to the previous release
* TODO: push new index.yaml generated to the chart branch