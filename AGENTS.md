# rhc-osdk-utils

## Project Overview

rhc-osdk-utils is a Go utility library providing reusable building blocks for Kubernetes operator
development. It is built on [controller-runtime][controller-runtime] and [client-go][client-go],
and is designed to be imported as a Go module by operators that use the Operator SDK. The library
contains four packages: a deferred-write resource cache (`resourceCache`), type-neutral resource
status checking (`resources`), common operator utilities (`utils`), and structured logging with
optional CloudWatch integration (`logging`).

## Dependencies

### Runtime

- **Go** 1.25+
- [controller-runtime][controller-runtime]
- [client-go][client-go]
- [k8s.io/api][k8s-api] and [k8s.io/apimachinery][k8s-apimachinery]
- [aws-sdk-go][aws-sdk-go] v1 (required by platform-go-middlewares v2)
- [platform-go-middlewares/v2][pgm] (CloudWatch logging)
- [go-logr/logr][logr] and [go-logr/zapr][zapr]
- [go.uber.org/zap][zap]
- [go-difflib][go-difflib] (debug diffs in resource cache)

### Dev / Test

- [testify][testify]
- [controller-runtime/pkg/envtest][envtest] (local Kubernetes API server for tests)
- [golangci-lint][golangci-lint] (run via Podman)
- [Podman](https://podman.io) (for linting)
- No build dependencies beyond the standard Go toolchain and Make

## Development Commands

See the [README][readme] for full details.

| Command | Description |
|---|---|
| `make test` | Run tests (requires envtest; also runs `fmt` and `vet`) |
| `make lint` | Run golangci-lint via Podman |
| `make fmt` | Format code with `go fmt` |
| `make vet` | Run `go vet` |

## Architecture

The library is organized into four packages. See [ARCHITECTURE.md][architecture] for detailed
descriptions, key types, component interactions, and data flow diagrams.

- **`resourceCache`** -- Deferred-write object cache. Key types: `ObjectCache`, `CacheConfig`,
  `ResourceIdent`, `ResourceIdentSingle`, `ResourceIdentMulti`, `Updater`, `GVKMap`.
- **`resources`** -- Type-neutral resource querying and status checking. Key types: `Resource`,
  `ResourceList`, `ResourceCounter`, `ResourceCounterResults`, `CommonGVKs`.
- **`utils`** -- Operator utilities. Key types: `Updater`, `MetaMutator`. Key functions:
  `UpdateOrErr`, `ApplyAll`, `GetKindFromObj`, `MakeOwnerReference`, `MakeLabeler`, `MakeService`,
  `MakePVC`, `CopySecret`, pointer helpers, random string generators, safe integer conversion.
- **`logging`** -- Structured logging. Key functions: `SetupLogging`, `SetupLoggingWithLevel`.

## Code Style

### Formatting

- Go standard formatting enforced via `goimports` (configured in `.golangci.yml` as a formatter).
- `go fmt` is run as part of `make test`.

### Linters

The following linters are enabled in `.golangci.yml` (golangci-lint config):

- `bodyclose` -- checks HTTP response body closure
- `errcheck` -- checks for unchecked errors
- `gocritic` -- various Go code checks
- `gosec` -- security-related checks
- `govet` -- reports suspicious constructs
- `ineffassign` -- detects ineffectual assignments
- `revive` -- extensible linter (with `exported` and `package-comments` rules disabled)
- `staticcheck` -- comprehensive static analysis
- `unused` -- checks for unused code

### Conventions

- Error wrapping uses `fmt.Errorf` with `%w` for error chains.
- Table-driven tests using `t.Run` with subtests.
- Pointer helper functions (`IntPtr`, `Int32Ptr`, `Int64Ptr`, `BoolPtr`, `TruePtr`, `FalsePtr`,
  `StringPtr`) are used extensively to create pointers to literal values for Kubernetes specs.
- The `Updater` type (a `bool`) encapsulates the create-or-update pattern: `true` means the
  resource exists and should be updated, `false` means it should be created.
- `nolint` directives with explanatory comments are used where necessary (e.g., the `logging`
  package uses `//nolint:staticcheck` on AWS SDK v1 imports with a comment explaining why).
- Resource identifiers use a provider/purpose string pair to namespace cached objects.

## Common Mistakes

1. **Missing envtest setup for `resourceCache` tests.** Tests in the `resourceCache` package
   require a running envtest environment (local Kubernetes API server). Running `go test ./...`
   without `KUBEBUILDER_ASSETS` set will cause test failures. Use `make test` which handles
   envtest setup automatically.

2. **Not registering GVKs in `CacheConfig` when using strict mode.** When `StrictGVK` is enabled
   in `Options`, every GVK must be present in the `possibleGVKs` map before calling
   `ObjectCache.Create`. Failing to register a GVK results in an error. Use
   `AddPossibleGVKFromIdent` or pass GVKs to `NewCacheConfig` upfront.

3. **Using AWS SDK v2 instead of v1.** The `logging` package depends on
   `platform-go-middlewares/v2` which requires `aws-sdk-go` v1 (not v2). Attempting to upgrade to
   `aws-sdk-go-v2` will break the CloudWatch logging integration. The `nolint:staticcheck`
   directives on the imports document this constraint.

4. **Misinterpreting the `Updater` bool.** `Updater(true)` means the resource **already exists**
   in the cluster and should be **updated**. `Updater(false)` means the resource was **not found**
   and should be **created**. Swapping these semantics causes either "resource already exists" or
   "resource not found" errors at apply time.

5. **Forgetting to add GVK schemes to the runtime scheme.** Every Kubernetes resource type used
   with the resource cache must have its API group added to the `runtime.Scheme` (e.g.,
   `clientgoscheme.AddToScheme`, `apps.AddToScheme`). Missing schemes cause `GetKindFromObj` to
   fail silently or return incorrect GVK values.

[controller-runtime]: https://pkg.go.dev/sigs.k8s.io/controller-runtime
[client-go]: https://pkg.go.dev/k8s.io/client-go
[k8s-api]: https://pkg.go.dev/k8s.io/api
[k8s-apimachinery]: https://pkg.go.dev/k8s.io/apimachinery
[aws-sdk-go]: https://pkg.go.dev/github.com/aws/aws-sdk-go
[pgm]: https://github.com/RedHatInsights/platform-go-middlewares
[logr]: https://pkg.go.dev/github.com/go-logr/logr
[zapr]: https://pkg.go.dev/github.com/go-logr/zapr
[zap]: https://pkg.go.dev/go.uber.org/zap
[go-difflib]: https://github.com/RedHatInsights/go-difflib
[testify]: https://pkg.go.dev/github.com/stretchr/testify
[envtest]: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest
[golangci-lint]: https://golangci-lint.run
[readme]: ./README.md
[architecture]: ./ARCHITECTURE.md
