# Architecture

## Overview

rhc-osdk-utils is a Go utility library that provides reusable building blocks for Kubernetes
operator development. It is designed to be imported as a module by operators built with the
[Operator SDK][operator-sdk] and [controller-runtime][controller-runtime]. The library addresses
common operator patterns such as deferred resource application, type-neutral resource status
checking, structured logging with CloudWatch integration, and miscellaneous Kubernetes object
helpers.

The library is organized into four independent packages, each targeting a specific concern in
operator development.

## Packages

### `resourceCache`

The core package of the library. It implements a deferred-write object cache that allows multiple
code paths (providers) to create, read, and modify Kubernetes resources before any of them are
written to the API server. Resources are tracked by a composite key of provider name, purpose
string, and object type.

**Key types:**

- `ObjectCache` -- The main cache struct. Holds a map of `ResourceIdent` to namespaced resources,
  a GVK-based resource tracker for reconciliation, a `*runtime.Scheme`, a
  `controller-runtime/pkg/client.Client`, and a `*CacheConfig`.
- `CacheConfig` -- Configuration for the cache including `possibleGVKs`, `protectedGVKs`, the
  runtime scheme, and an `Options` struct.
- `Options` -- Cache behavior options: `StrictGVK` (bool), `Ordering` (string slice for apply
  order), and `DebugOptions`.
- `DebugOptions` -- Toggles for logging at `Create`, `Update`, `Apply`, and `Registration` stages.
- `ResourceIdent` -- Interface with methods `GetProvider()`, `GetPurpose()`, `GetType()`, and
  `GetWriteNow()`.
- `ResourceIdentSingle` -- Implements `ResourceIdent` for single-item-per-ident entries.
- `ResourceIdentMulti` -- Implements `ResourceIdent` for multi-item-per-ident entries.
- `ResourceOptions` -- Optional configuration passed to `NewSingleResourceIdent` and
  `NewMultiResourceIdent` (currently contains `WriteNow` bool).
- `k8sResource` -- Internal struct holding the `client.Object`, an `Updater` (create-or-update
  flag), a `Status` bool, JSON debug data, and the original object for diff comparison.
- `GVKMap` -- Type alias `map[schema.GroupVersionKind]bool` used for possible and protected GVK
  sets.
- `ObjectToApply` / `objectsToApply` -- Sorting infrastructure for ordered resource application.

**Key operations:**

| Method | Description |
|---|---|
| `NewCacheConfig` | Creates a `CacheConfig` with scheme, GVK maps, and options |
| `NewObjectCache` | Instantiates an `ObjectCache` from a context, client, logger, and config |
| `Create` | Fetches a resource from the cluster (or uses a blank), stores it in the cache |
| `Update` | Replaces the cached copy; optionally writes immediately if `WriteNow` is set |
| `Get` | Retrieves a cached resource by ident (single) or by ident + `NamespacedName` (multi) |
| `List` | Returns all resources for a `ResourceIdentMulti` as an `UnstructuredList` |
| `Status` | Marks a resource for status subresource update during apply |
| `ApplyAll` | Sorts resources by configured ordering, then creates or updates each in the cluster |
| `Reconcile` | Deletes cluster resources whose GVK is in `possibleGVKs` but not in the cache |
| `AddPossibleGVKFromIdent` | Registers GVKs from resource idents into the possible set |

### `resources`

Provides a type-neutral abstraction over Kubernetes resources for status checking and counting.
Works with `unstructured.Unstructured` objects so that any resource kind can be represented without
requiring generated Go types.

**Key types:**

- `Resource` -- Represents any Kubernetes resource, parsed from an `Unstructured` object. Contains
  `ResourceMetadata`, `ResourceStatus`, `Conditions`, and `ReadyRequirements`.
- `ResourceMetadata` -- Parsed metadata: `Generation`, `Namespace`, `Name`, `UID`,
  `ResourceVersion`, `OwnerUIDs`.
- `ResourceStatus` -- Parsed status containing `ObservedGeneration`.
- `ResourceConditionReadyRequirements` -- Defines the condition `Type` and `Status` that indicate
  readiness.
- `ResourceList` -- A collection of `Resource` instances with methods for counting, filtering by
  owner, bucketing by ready/broken status, and querying by GVK and namespace.
- `ResourceStatusBuckets` -- Groups resources into `Ready` and `Broken` slices.
- `ResourceCounter` -- High-level API that queries resources across namespaces, checks readiness,
  and returns `ResourceCounterResults`.
- `ResourceCounterResults` -- Result struct with `Managed`, `Ready` counts and a
  `BrokenMessage` string.
- `ResourceCounterQuery` -- Query parameters: `GVK`, `Namespaces`, `OwnerGUID`.
- `GVKs` -- Struct holding common `GroupVersionKind` values for `Deployment`, `Kafka`,
  `KafkaTopic`, and `KafkaConnect`.
- `CommonGVKs` -- Pre-defined instance of `GVKs` with standard group/version/kind values.

### `utils`

General-purpose Kubernetes operator utilities. Includes the `Updater` type that encapsulates the
create-or-update pattern, random string generators, pointer helper functions, resource construction
helpers, label/annotation management, and safe integer conversion.

**Key types and functions:**

- `Updater` -- A `bool` type where `true` means update and `false` means create. Its `Apply`
  method calls `client.Update` or `client.Create` accordingly.
- `UpdateOrErr` -- Converts a `client.Get` error into an `Updater` value: `true` if the resource
  exists, `false` if not found.
- `UpdateAllOrErr` -- Batch version of `UpdateOrErr` for multiple objects.
- `ApplyAll` -- Applies a map of objects to their corresponding `Updater` values.
- `MetaMutator` -- Interface for objects supporting annotation and label get/set operations.
- `UpdateAnnotations` / `UpdateLabels` -- Merge annotations or labels into an object.
- `MakeOwnerReference` -- Creates an `OwnerReference` from a `client.Object`.
- `MakeLabeler` / `GetCustomLabeler` -- Returns labeler functions that apply name, namespace,
  labels, and owner references to objects.
- `MakeService` -- Configures a `core.Service` with labels, ports, and optional NodePort.
- `MakePVC` -- Configures a `PersistentVolumeClaim` with labels and storage size.
- `GetKindFromObj` -- Retrieves the `GroupVersionKind` for a registered runtime object.
- `CopySecret` -- Copies a `Secret` from one `NamespacedName` to another.
- Pointer helpers: `IntPtr`, `Int32Ptr`, `Int64Ptr`, `BoolPtr`, `TruePtr`, `FalsePtr`,
  `StringPtr`.
- Random string generators: `RandString`, `RandStringLower`, `RandHexString`, `RandPassword`.
- Integer conversion: `Int32`, `Atoi32` (safe int-to-int32 and string-to-int32).
- String utilities: `Contains`, `IntMin`, `IntMax`, `ListMerge`, `B64Decode`.

### `logging`

Configures structured logging with [zap][zap] and optional [CloudWatch][cloudwatch] integration
via [platform-go-middlewares][pgm].

**Key functions:**

- `SetupLogging` -- Creates a `*zap.Logger` that accepts all log levels, with console JSON output
  and optional CloudWatch streaming.
- `SetupLoggingWithLevel` -- Same as `SetupLogging` but filters logs below a specified level.
- `buildCore` (internal) -- Constructs a `zapcore.Core` with console and optional CloudWatch
  writers. CloudWatch is enabled when `AWS_CW_KEY`, `AWS_CW_SECRET`, `AWS_CW_LOG_GROUP`, and
  `AWS_CW_REGION` environment variables are set.

## Component Interactions

```
resourceCache
    depends on --> utils (Updater, GetKindFromObj)
    depends on --> controller-runtime/pkg/client
    depends on --> k8s.io/apimachinery (runtime, schema, types, unstructured)
    depends on --> go-difflib (debug diffs)

resources
    depends on --> controller-runtime/pkg/client
    depends on --> k8s.io/apimachinery (unstructured, runtime, schema, types)

utils
    depends on --> controller-runtime/pkg/client
    depends on --> k8s.io/apimachinery (runtime, schema, types, resource)
    depends on --> k8s.io/api/core/v1

logging
    depends on --> aws-sdk-go v1 (credentials, config)
    depends on --> platform-go-middlewares/v2 (CloudWatch batch writer)
    depends on --> go.uber.org/zap
```

The `resourceCache` package is the primary consumer of `utils`, specifically using the `Updater`
type to determine whether cached resources should be created or updated when applied to the
cluster, and `GetKindFromObj` to resolve GVK information from runtime objects. The `resources` and
`logging` packages are independent of each other and of `resourceCache`.

## Resource Cache Data Flow

1. **Initialization** -- `NewCacheConfig` creates a `CacheConfig` with a scheme, GVK maps, and
   options. `NewObjectCache` creates the cache with an empty `data` map and `resourceTracker`.

2. **Create phase** -- Each provider calls `ObjectCache.Create` with a `ResourceIdent`,
   `NamespacedName`, and empty `client.Object`. The cache calls `client.Get` to fetch the current
   cluster state. The result is wrapped in a `k8sResource` with an `Updater` flag indicating
   whether the resource already exists. A deep copy is stored as `origObject` for later diff
   comparison.

3. **Update phase** -- Providers call `ObjectCache.Get` to retrieve cached resources, modify them,
   then call `ObjectCache.Update` to write changes back to the cache. If the `ResourceIdent` has
   `WriteNow` set, the resource is applied immediately to the cluster during this phase.

4. **Apply phase** -- `ObjectCache.ApplyAll` collects all cached resources, sorts them by the
   configured `Ordering` (defaulting to `*`, `Deployment`, `Job`, `CronJob`), and applies each
   one. Before applying, each resource is compared against its `origObject` using
   `equality.Semantic.DeepEqual`. If unchanged and the resource already existed (`Updater` is
   `true`), the apply is skipped to reduce API calls. Resources marked for status updates have
   their status subresource updated after the main apply.

5. **Reconcile phase** -- `ObjectCache.Reconcile` iterates over all GVKs in `possibleGVKs` (minus
   `protectedGVKs`), lists cluster resources of each kind, and deletes any owned resource not
   present in the cache. This garbage-collects resources that are no longer managed.

[operator-sdk]: https://sdk.operatorframework.io
[controller-runtime]: https://pkg.go.dev/sigs.k8s.io/controller-runtime
[zap]: https://pkg.go.dev/go.uber.org/zap
[cloudwatch]: https://aws.amazon.com/cloudwatch/
[pgm]: https://github.com/RedHatInsights/platform-go-middlewares
