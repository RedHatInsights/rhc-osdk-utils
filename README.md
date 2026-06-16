# rhc-osdk-utils

A collection of reusable Go utilities for Kubernetes operator development, built on top of
[controller-runtime][controller-runtime] and [client-go][client-go]. Provides a deferred-write
resource cache, type-neutral resource status checking, structured logging with optional CloudWatch
integration, and common Kubernetes object helpers.

## Installation

```bash
go get github.com/RedHatInsights/rhc-osdk-utils
```

## Packages

### resourceCache

A deferred-write object cache that lets multiple providers create, read, and modify Kubernetes
resources before applying them all at once. Reduces API calls, prevents unnecessary pod restarts,
and supports automatic cleanup of unmanaged resources.

```go
import (
    rc "github.com/RedHatInsights/rhc-osdk-utils/resourceCache"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    core "k8s.io/api/core/v1"
)

// Set up a scheme with the required GVKs
scheme := runtime.NewScheme()
utilruntime.Must(clientgoscheme.AddToScheme(scheme))

// Create cache config and cache
config := rc.NewCacheConfig(scheme, nil, nil)
cache := rc.NewObjectCache(ctx, k8sClient, &logger, config)

// Create a resource ident and populate the cache
ident := rc.NewSingleResourceIdent("myProvider", "configMap", &core.ConfigMap{})
nn := types.NamespacedName{Name: "my-config", Namespace: "default"}
cm := &core.ConfigMap{}
err := cache.Create(ident, nn, cm)

// Modify and update
cm.Data = map[string]string{"key": "value"}
err = cache.Update(ident, cm)

// Apply all cached resources to the cluster
err = cache.ApplyAll()

// Clean up resources no longer in the cache
err = cache.Reconcile(ownerUID, client.InNamespace("default"))
```

### resources

Type-neutral resource querying and status checking using `unstructured.Unstructured` objects.
Count managed resources, check readiness, and filter by owner.

```go
import "github.com/RedHatInsights/rhc-osdk-utils/resources"

// Count deployments across namespaces owned by a specific UID
counter, err := resources.MakeResourceCounterForType(
    &apps.Deployment{},
    *scheme,
    []string{"namespace-a", "namespace-b"},
    ownerUID,
    []resources.ResourceConditionReadyRequirements{{
        Type:   "Available",
        Status: "True",
    }},
)

results, err := counter.Count(ctx, k8sClient)
fmt.Printf("Managed: %d, Ready: %d\n", results.Managed, results.Ready)
```

### utils

Common operator utilities including the `Updater` create-or-update pattern, pointer helpers,
random string generators, label/annotation management, and safe integer conversion.

```go
import "github.com/RedHatInsights/rhc-osdk-utils/utils"

// Determine whether to create or update a resource
updater, err := utils.UpdateOrErr(k8sClient.Get(ctx, nn, obj))

// Apply the resource (creates if new, updates if existing)
err = updater.Apply(ctx, k8sClient, obj)

// Pointer helpers for Kubernetes specs
replicas := utils.Int32Ptr(3)
enableFlag := utils.TruePtr()

// Generate random strings
password, err := utils.RandPassword(20)
hexID := utils.RandHexString(8)
```

### logging

Structured logging with [zap][zap] and optional [CloudWatch][cloudwatch] integration.

```go
import "github.com/RedHatInsights/rhc-osdk-utils/logging"

// Basic setup (all levels enabled)
logger, err := logging.SetupLogging(false)
defer logger.Sync()

// With level filtering (e.g., info and above)
logger, err := logging.SetupLoggingWithLevel(false, 0)
```

CloudWatch is enabled automatically when the `AWS_CW_KEY`, `AWS_CW_SECRET`, `AWS_CW_LOG_GROUP`,
and `AWS_CW_REGION` environment variables are set. Optionally set `AWS_CW_ENDPOINT` for custom
endpoints.

## Development

This project uses a standard Go toolchain with [Make][make] for common tasks.

### Prerequisites

- Go (see `go.mod` for version)
- [Podman][podman] (for linting)
- [envtest][envtest] (for testing -- automatically installed via Make)

### Commands

| Command | Description |
|---|---|
| `make test` | Run tests with envtest, format, and vet |
| `make lint` | Run golangci-lint via Podman container |
| `make fmt` | Format code with `go fmt` |
| `make vet` | Run `go vet` |

### Running tests

Tests in the `resourceCache` package require [envtest][envtest] to provide a local Kubernetes API
server. The `make test` target handles envtest setup automatically.

```bash
make test
```

## Contributing

Contributions are welcome. Please open an issue or pull request on [GitHub][repo].

## License

This project is licensed under the [Apache License 2.0][license].

[controller-runtime]: https://pkg.go.dev/sigs.k8s.io/controller-runtime
[client-go]: https://pkg.go.dev/k8s.io/client-go
[zap]: https://pkg.go.dev/go.uber.org/zap
[cloudwatch]: https://aws.amazon.com/cloudwatch/
[make]: https://www.gnu.org/software/make/
[podman]: https://podman.io
[envtest]: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
[repo]: https://github.com/RedHatInsights/rhc-osdk-utils
[license]: ./LICENSE
