# linkerd-dataplane-restarter

This CLI app slowly restarts all deployments in `default` namespace for which `linkerd-proxy` version differs from
`linkerd-proxy-injector` from Control Plane. It aids the process of restarting all running deployments by minimizing the
need for i.a. scaling while the restart of dependent services is in progress.

## Example scenario

Imagine that you upgraded Linkerd Control Plane from version edge-21.9.1 to stable-2.11.0. All your Data Planes are
still running with the old version. In order to change this you would need to restart all running deployments. If you
know k8s, you might know that to do this you could just run `kubectl rollout restart deployment <your-deployment-name>`
which would restart <your-deployment-name>.

You could also run `kubectl rollout restart deployment`, but this command restarts all deployment instantly.

The problem with this solution is that this requires a lot of resources and if your apps call other apps on start,
others apps would scale unnecessary or be killed if wrongly configured.

In this scenario this app comes in handy. It checks which deployments require restarting and restarts them one-by-one
with additional sleep between if wanted.

## Installation

```shell
go install github.com/shipwallet/public-tools/linkerd-dataplane-restarter@latest
```

## Usage

```shell
$ linkerd-dataplane-restarter -h

Usage of linkerd-dataplane-restarter:
  -s, --sleep duration     how long to wait between each deployment restart (default 1m0s)
  -t, --timeout duration   how long to wait for deleting pods (default 10m0s)
```
