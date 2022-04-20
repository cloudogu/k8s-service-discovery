# Guide to development

## Local Deploy

1. the environment variable `KUBECONFIG` must have a valid config for the target cluster.
1. export the environment variable `WATCH_NAMESPACE`: `export WATCH_NAMESPACE=ecosystem`.
1. run `make run` to execute the service discovery operator locally.

## Makefile targets

The `make help` command prints all available targets and their descriptions on the command line.