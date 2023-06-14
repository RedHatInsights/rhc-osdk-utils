[![codecov](https://codecov.io/gh/RedHatInsights/rhc-osdk-utils/branch/master/graph/badge.svg?token=H8CT10UQ3L)](https://codecov.io/gh/RedHatInsights/rhc-osdk-utils)

# Red Hat Cloud Operator SDK Utils
### A handy collection of helpful utils for everyday operator development.
This document is broken down by package, providing examples of the various types and public APIs.

## Resource Cache
### Background Problem Statement
The Resource Cache is a handy utility for working with k8s resources in situations where you need
multiple blocks of code to have access to resources before they are written to k8s, without having
to explicitly pass the objects around. Consider the following, a `Deployment` resource needs to be
updated by three blocks of code. Traditionally either the resources would need to be passed
successively to the three blocks of code, or the resource would need to be written to k8s and then
*got*, and *updated* by each subsequent block.

In the first instance, the code is not only difficult to follow and maintain, but requires knowing
explicitly which resources are needing to be passed to the calling functions. In the second instance,
multiple writes need to be made to k8s meaning that either, the API gets excessive calls, or that,
in the case of the deployment, a pod could be restarted multiple times needlessly.

### The Resource Cache as a solution
The resource cache solves this problem by holding a cache of objects, organised by type, that can be
retrieved by name, or as a list and then updated. Once all code blocks are completed, everything
which is in the cache is written to k8s in a large block. This has other benefits too, stated thus:

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

# Creation of logger is omitted

config := NewCacheConfig(scheme, nil, nil)

ctx = context.Background()

oCache := NewObjectCache(ctx, k8sClient, &config)
```

A scheme object **must** be created and should be expected to contain every GVK that will be used in
the cache. If an object is attempted to be added to the cache and it does not exist in the scheme,
**problems will occur**. The `init()` function here shows the creation of a scheme and the addition
of two schemas to it. One from the core client-go library for k8s and the other from the deployments
schema.

Next two empty maps are passed in, the possible and protected GVK. As the Resource Cache has the
ability to delete resources, some GVKs can be protected to prevent this from happening. For example
you may want to protect the `deployment` type resource. In which case, passing in a GVK object to
the protectedGVKs map with a bool value of true will prevent that from happening.

#### Creating an object in the cache
Once a cache object is instantiated the Cache can be read from and written to as in the example
below:

```go
	nn := types.NamespacedName{
		Name:      "test-service",
		Namespace: "default",
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "setup",
		Purpose:  "core_service",
		Type:     &core.Service{},
	}

	err := oCache.Create(SingleIdent, nn, &s)
```

In this example a k8s `Service` resource is created in the cache. Note the `Create()` call takes a
pointer to a Service object. On creation, the Resource Cache will go and query k8s to find that
resource and if it exists, populate the object `s`. If it doesn't exist, the object remains
unmodified.

The purpose of the `ResourceIdentSingle`, is to provide the cache with a way of indexing this
resource. In this way, it is known that this particular resource was created by the `setup`
provider, (or block of code), and its purpose is as the `core_service`. The use of
`ResourceIdentSingle` means that only one of these will ever exist in the cache for this
provider/purpose combination.

There is a `ResourceIdentMulti` which allows for putting multiple items with the same provider and
purpose. A use case for this may be that you need to create multiple variations of a `Service`
resource, but you have another block of code that needs to get *all* of these and modify them,
perhaps adding a `ClusterIP`, without having to know how they are named. Doing this in the cache is
beneficial, as otherwise annotations would need to be added to the object if it were to be
distinguishable from other items in k8s.

#### possibleGVKs and strict mode
When the Resource Cache is working in the default mode, GVKs are added to the possibleGVKs map *as
they are created*. While this is the most optimal way to collect which GVKs to use for the
`Reconcile()` stage, it leaves a bug in the system. An existing resource of kind X, will never be
removed if the reconciliation no longer *creates* any item of type X, as the creation is the method
of populating the `possibleGVKs` list.

The Resource Cache allows for passing in an option flag to the config, which enables a strict mode.
In this mode, the possibleGVKs **must** populated ahead of time. The `Create()` call will fail if
strict mode is enabled and the kind of the object is not in the `possibleGVKs` list. You can also
add GVKs to the list using the `AddPossibleGVKFromIdent()` function.

#### Updating an item in the cache
Once the item has been `created` (or initialised) we move on to updating the object. If we add data
to the object before the create, it could be overwritten if the object exists for real in k8s. Here
the object is updated with a service port, before calling the `Update()` function.

```go
	s.Spec = core.ServiceSpec{
        Ports: []core.ServicePort{{
            Name: "port-01",
            Port: 5432,
        }},
    }

	err = oCache.Update(SingleIdent, &s)
```

We use the ident `SingleIdent` to tell k8s which object to update. This, along with the namespaced
name information, is enough to locate the object, and the version passed in, replaces the version in
the cache.

#### Getting an item in the cache in another code block
Another code block now may need to access that item to perform other operations on it. To do so it
needs to be retrieved from the cache. This is performed using the `Get()` call.

```go
	newService := core.Service{}

	err = oCache.Get(SingleIdent, &newService)
```

Notice here a namespaced name is not supplied. It is known that this is a single object and the
*ident* is enough. The `newService` variable is *filled* with the data from the cache. This can then
be modified and written back with another `Update()` call. Note that unless the `Update()` call is
made, the changes will not appear in the cache and will die with garbage collection at some point.

#### Applying the cache

Once all the changes have been made to resources, the cache can be applied using the `ApplyAll()`
function. It is important to remember that the cache is clean at every reconciliation. You should
not persist the cache across multiple reconciliations or the resource deletion will not work.

```go
	err = oCache.ApplyAll()
```

This will write everything in the cache out to k8s in a single operation.

#### Reconciling the cache

Once the cache has been written an optional step can be run call `Reconcile()`. This function lists
every resource that has a kind listed in `possibleGVKs` and looks for ownership from a specific
resource. This helps guard against deleting resources that are not created by the code in question.
The list is iterated through and any resource which is **not** in the cache will be deleted. This
works because a reconciliation should ensure that **every** resource they care about is
written/updated.

Objects which have a k8s kind in the `protectedGVK` list will not be deleted by the Resource Cache.

### Optimisations
Certain optimisations are present to speed things up and help with optimisation.

* When an item is retrieved from the cache, during the `Create()` call, a copy is stored. When
  alterations to the object are made through `Update()`, these are made to a new copy of the
  resource. When an apply is made, the Resource Cache compares the *initial version* to the *updated
  version*. If there is no change, then nothing is written to k8s and hence API calls are saved.
  This does come an some expense to the Resource Cache's resources, but in practice it is often far
  more beneficial to spend cycles checking this than making thousands of unnecessary writes to k8s.

* The Resource Cache sets a flag on the object inside the cache when the `Create()` is used so that
  it is aware if the object needs to be created or updated. Usually working with the k8s client
  requires the developer to know this or check for it, the Resource Cache makes this step
  unnecessary.

* The `Reconcile()` function can be costly as it needs to list every object of every kind in the
  `possibleGVKs` list. To minimise this, options can be passed to Reconcile to allow it to filter
  objects. This is often used to filter to within a certain namespace, or a certain label.

* Certain objects are always written before others, or rather certain objects are always written
  last. `Deployments/Jobs` are always written at the end of the `ApplyAll()` because they often rely
  on other objects to be written out. Particularly if the reconciliation is happening for the first
  time, these dependent objects may not exist and will cause a deployment to spin up pods which will
  fail with a `ConfigCreateError`. Ordering resources like this allows for the best chance of
  successful pod deployment.

### WriteNow support
There are some situations where a resource requires to be written immediately and not wait for an
`ApplyAll()`. In this case, it will be skipped in the `ApplyAll()` step as an `Update()` call will
immediately update it in k8s. This should be used with care. The object should not be updated after
the initial update, though there is nothing to prevent this from happening, it ill simply result in
a second API write.

```go
NewSingleResourceIdent("prov", "purpose", &core.ConfigMap{}, rc.ResourceOptions{WriteNow: true})
```

### Custom apply ordering
Sometimes there are situations where you want to order the application of resources to the cluster,
that is, certain types should be applied first. Applying a ConfigMap after a Deployment that relies
on it, can cause the Deployment to initially fail. The ordering can be supplied as follows:

```go
var applyOrder []string = []string{
	"*",
	"Deployment",
	"Job",
	"CronJob",
	"ScaledObject",
}

config := NewCacheConfig(scheme, nil, nil, Options{
  Ordering: applyOrder,
})
```

### Debugging
There is a debug options struct which can be passed to the `config.Options` enabling independent
logging for `create`, `update` and `apply` operations.

## Utils
TODO

## Resources
The resources package provides a type neutral way to query, count, and check the status of k8s
resources. The k8s operator SDK provides many rich ways of doing these tasks. However, these methods
are either strongly typed, involve parsing raw JSON, or are esoteric. This package attempts to
provide an API for performing common resource evaluation tasks without the difficulties of many
differing types and without the performance overhead of working with JSON.

### ResourceCounter
A common operator pattern is to get a collection of a specific type of resource, get a ready status
on those resources, and then produce a count of what's managed, what's ready, and what's broken. The
`resources.ResourceCounter` type provides that functionality.

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
* The results will be a `resources.ResourceCounterResults` struct which shows the number of managed
  and ready resources and a report on the broken ones.
* `ReadyRequirements` is set to a `resources.ResourceConditionReadyRequirements`. This type defines
  the criteria used to decide that a resource is ready. This is done by looking for a status
  condition on the resource that matches the provided type and status. This unfortunately differs
  from resource to resource; for example, a deployment wants type `"Available"` and status `"True"`
  while Kafka wants type `"Ready"` and status `"True"`

### MakeQuery
You may want to write a query for an object you don't know the GVK for and that we don't have in
`CommonGVKs`. For that you can use the `MakeQuery` function which will accept a `runtime.Scheme`
along with other query requirements to build a query. In the example below we pass `MakeQuery` a
type specimen - in this case an `apps.Deployment{}` along with a `runtime.Scheme`, namespace string
list and a GUID. The resulting query can be used by `ResourceCounter`.

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
A common pattern is what we saw above: make a query for a type and then generate a ResourceCounter
for it. The `MakeResourceCounterForType` method will collapse that into one step for you:

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
ResourceCounter abstracts away ResourceList and provides an API for a bunch of common operations. If
you want to perform your own logic with a list of resources you'd want to use ResourceList:

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

The code above would populate the resource list with resources of a given GVK for a given namespace.
We provide a struct with common GVKs for things we often status check:

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
See the code for the full public API. Methods are provided for counting and filtering the list by
various criteria, etc.

### Resource
This type (hopes to be able to) represent any resource that k8s can manage. It works by
parsing `unstructured.Unstructured` k8s objects and representing them as simple to use Go types
rather than esoteric nested maps of interfaces. Because it uses `Unstructured` as its source it
should be as performant as native k8s resource types, but is significantly easier to use. There's no
way to get a Resource instance on its own; you use `ResourceList` to run a query and then can get
individual resources from that.

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
