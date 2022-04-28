# Red Hat Cloud Operator SDK Utils
### A handy collection of helpful utils for everyday operator development.
This document is broken down by package, providing examples of the various types and public APIs.

## Resource Cache
TODO

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
        GVK: schema.GroupVersionKind{
            Group:   "apps",
            Kind:    "Deployment",
            Version: "v1",
        },
        OwnerGUID: guid,
    },
    ReadyRequirements: resources.ResourceConditionReadyRequirements{
        Type:   "Available",
        Status: "True",
    },
}

results := counter.Count(context, client)
```
Somethings worth noting in the above example:
* The results will be a `resources.ResourceCounterResults` struct which shows the number of maanaged and ready resources and a report on the broken ones.
* `ReadyRequirements` is set to a `resources.ResourceConditionReadyRequirements`. This type defines the criteria used to decide that a resource is ready. This is done by looking for a status condition on the resource that matches the provided type and status. This unfortunately differs from resource to resource; for example, a deployment wants type `"Available"` and status `"True"` while Kafka wants type `"Ready"` and status `"True"`

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

The code above would populate the resource list with resources of a given GVK for a given namespace. A number of useful operations could then be performed on the list:

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