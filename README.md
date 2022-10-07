# Red Hat Cloud Operator SDK Utils
### A handy collection of helpful utils for everyday operator development.
This document is broken down by package, providing examples of the various types and public APIs.

## Resource Cache
### Background Problem Statement
The Resource Cache is a handy utility for working with k8s resources in situations where you need multiple blocks of code to have access to 
resources before they are written to k8s, without having to explicitly pass the objects around. Consider the following, a `Deployment` resource needs to be updated by three blocks of code. Traditionally either the resources would need to be passed successively to the three blocks of code, or the resource would need to be written to k8s and then *got*, and *updated* by each subsequent block.

In the first instance, the code is not only difficult to follow and maintain, but requires knowing explcitly which resourcs are needing to be passed to the calling functions. In the second instance, multiple writes need to be made to k8s meaning that either, the API gets excessive calls, or that, in the case of the deployment, a pod could be restarted multiple times needlessly.

### The Resource Cache as a solution
The resource cache solves this problem by holding a cache of objects, organized by type, that can be retrieved by name, or as a list and then updated. Once all code blocks are completed, everything which is in teh cache is written to k8s in a large block. This has other benefits too, stated thus:

* less API calls to k8s
* the blocks of code can operate on multiple resources of the same type without knowing their names
* resources will only be written once generation code is complete and error free
* cleaning up of unused resources is possible

### Using the Resource Cache
The Resource Cache is created via a config object.

```go
var (
	scheme   = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apps.AddToScheme(scheme))
}

## Creationg of logger is omitted

config := NewCacheConfig{
    scheme:        scheme,
    possibleGVKs:  make(GVKMap),
    protectedGVKs: make(GVKMap),
}

ctx = context.WithValue(ctx, Key("context"), &log)

oCache := NewObjectCache(ctx, k8sClient, &config)
```

A scheme object **must** be created and should be expected to contain every GVK that will be used in the cache. If an object is attempted to be added to the cache and it does not exist in the scheme, **problems will occur**. The `init()` function here shows the creation of a scheme and the addition of two schemas to it. One from the core client-go library for k8s and the other from the deployments schema.

Next two empty maps are passed in


## Utils
TODO

## Resources
The resources package provides a type neutral way to query, count, and check the status of k8s resources. The k8s operator SDK provides many rich ways of doing these tasks. However, these methods are either strongly typed, involve parsing raw JSON, or are esoteric. This package attempts to provide an API for performing common resource evaluation tasks without the difficulties of many differing types and without the performance overhead of working with JSON.

### ResourceCounter
A common operator pattern is to get a collection of a specific type of resource, get a ready status on those resources, and then produce a count of what's managed, what's ready, and what's broken. The `resources.ResourceCounter` type provides that functionality.

This example counts all deployments in a list of namespaces and owned by a specific GUID.

```go
namespaces := []string{"SomeNS", "SomeOtherNS"}
guid := "2we34-32ed3-33d33-23rd3"
counter := resources.ResourceCounter{
    Query: resources.ResourceCounterQuery{
        Namespaces: namespaces,
        GVK:     CommonGVKs.Deployment,
        OwnerGUID: guid,
    },
    ReadyRequirements: []resources.ResourceConditionReadyRequirements{
        {
            Type:   "Available",
            Status: "True",
        }
    },
}

results := counter.Count(context, client)
```
Somethings worth noting in the above example:
* The results will be a `resources.ResourceCounterResults` struct which shows the number of maanaged and ready resources and a report on the broken ones.
* `ReadyRequirements` is set to a `resources.ResourceConditionReadyRequirements`. This type defines the criteria used to decide that a resource is ready. This is done by looking for a status condition on the resource that matches the provided type and status. This unfortunately differs from resource to resource; for example, a deployment wants type `"Available"` and status `"True"` while Kafka wants type `"Ready"` and status `"True"`

### MakeQuery
You may want to write a query for an object you don't know the GVK for and that we don't have in `CommonGVKs`. For that you can use the `MakeQuery` function which will accept a `runtime.Scheme` along with other query requirements to build a query. In the example below we pass `MakeQuery` a type specimen - in this case an `apps.Deployment{}` along with a `runtime.Scheme`, namespace string list and a GUID. The resulting query can be used by `ResourceCounter`.

```go
query, _ := resources.MakeQuery(&apps.Deployment{}, *scheme, namespaces, o.GetUID())

counter := resources.ResourceCounter{
    Query: query,
    ReadyRequirements: []resources.ResourceConditionReadyRequirements{{
        Type:   "Available",
        Status: "True",
    }},
}
```

### MakeResourceCounterForType
A common pattern is what we saw above: make a query for a type and then generate a ResourceCounter for it. The `MakeResourceCounterForType` method will collapse that into one step for you:

```go
    counter := resources.MakeResourceCounterForType(
        &apps.Deployment{}, 
        *scheme, 
        namespaces, 
        o.GetUID(),
        []resources.ResourceConditionReadyRequirements{{
            Type:   "Available",
            Status: "True",
         }}
    )
```

### ResourceList
ResourceCounter abstracts away ResourceList and provides an API for a bunch of common operations. If you want to perform your own logic with a list of resources you'd want to use ResourceList:

```go
gvk := schema.GroupVersionKind{
            Group:   "apps",
            Kind:    "Deployment",
            Version: "v1",
        }
namespace := "SomeNS"
deployments := ResourceList{}
deployments.GetByGVKAndNamespace(client, context, namespace, gvk)
```

The code above would populate the resource list with resources of a given GVK for a given namespace. We provide a struct with common GVKs for things we often status check:

```go
	gvk := resources.CommonGVKs.Deployment
	resources.GetByGVKAndNamespace(pClient, ctx, namespace, gvk)
```

A number of useful operations could then be performed on the list:

```go
reqs := resources.ResourceConditionReadyRequirements{
            Type:   "Available",
            Status: "True",
        }
//Provide the resource list requirements for determining what
//resources are ready
deployments.SetReadyRequirements(reqs)
//Get an array of broken deployment resource objects
brokenDeployments := deployments.GetBrokenResources()
//Get an array of ready deployment resource objects
readyDeployments := deployments.GetReadyDeployments()

```
See the code for the full public API. Methods are provided for counting and filtering the list by various criteria, etc.

### Resource
This type (hopes to be able to) represent any resource that Kubernetes can manage. It works by parsing `unstructured.Unstructured` k8s objects and representing them as simple to use Go types rather than esoteric nested maps of interfaces. Because it uses `Unstructured` as its source it should be as performant as native k8s resource types, but is significantly easier to use. There's no way to get a Resource instance on its own; you use `ResourceList` to run a query and then can get individual resources from that.

```go
gvk := schema.GroupVersionKind{
            Group:   "apps",
            Kind:    "Deployment",
            Version: "v1",
        }
namespace := "SomeNS"
deployments := ResourceList{}
deployments.GetByGVKAndNamespace(client, context, namespace, gvk)

resource := deployments.GetResourceByIndex(0)

name := resource.Metadata("name")
namespace := resource.Metadata("namespace")

resource.SetReadyRequirements(resources.ResourceConditionReadyRequirements{
            Type:   "Available",
            Status: "True",
})

ready := resource.IsReady()
```
See the code for the full public API.




---

<sub>Made with ❤️ @ Red Hat</sub>