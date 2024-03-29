package resourcecache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/RedHatInsights/go-difflib/difflib"
	"github.com/RedHatInsights/rhc-osdk-utils/utils"
	"github.com/go-logr/logr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	core "k8s.io/api/core/v1"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/api/equality"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceIdent interface {
	GetProvider() string
	GetPurpose() string
	GetType() client.Object
	GetWriteNow() bool
}

type ResourceOptions struct {
	WriteNow bool
}

// ResourceIdent is a simple struct declaring a providers identifier and the type of resource to be
// put into the cache. It functions as an identifier allowing multiple objects to be returned if
// they all come from the same provider and have the same purpose. Think a list of Jobs created by
// a Job creator.
type ResourceIdentSingle struct {
	Provider string
	Purpose  string
	Type     client.Object
	WriteNow bool
}

func (r ResourceIdentSingle) GetProvider() string {
	return r.Provider
}

func (r ResourceIdentSingle) GetPurpose() string {
	return r.Purpose
}

func (r ResourceIdentSingle) GetType() client.Object {
	return r.Type
}

func (r ResourceIdentSingle) GetWriteNow() bool {
	return r.WriteNow
}

// ResourceIdent is a simple struct declaring a providers identifier and the type of resource to be
// put into the cache. It functions as an identifier allowing multiple objects to be returned if
// they all come from the same provider and have the same purpose. Think a list of Jobs created by
// a Job creator.
type ResourceIdentMulti struct {
	Provider string
	Purpose  string
	Type     client.Object
	WriteNow bool
}

func (r ResourceIdentMulti) GetProvider() string {
	return r.Provider
}

func (r ResourceIdentMulti) GetPurpose() string {
	return r.Purpose
}

func (r ResourceIdentMulti) GetType() client.Object {
	return r.Type
}

func (r ResourceIdentMulti) GetWriteNow() bool {
	return r.WriteNow
}

var secretCompare schema.GroupVersionKind

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	secretCompare, _ = utils.GetKindFromObj(scheme, &core.Secret{})
}

// NewSingleResourceIdent is a helper function that returns a ResourceIdent object.
func NewSingleResourceIdent(provider string, purpose string, object client.Object, opts ...ResourceOptions) ResourceIdentSingle {
	writeNow := false
	for _, opt := range opts {
		writeNow = opt.WriteNow
	}
	return ResourceIdentSingle{
		Provider: provider,
		Purpose:  purpose,
		Type:     object,
		WriteNow: writeNow,
	}
}

// NewMultiResourceIdent is a helper function that returns a ResourceIdent object.
func NewMultiResourceIdent(provider string, purpose string, object client.Object, opts ...ResourceOptions) ResourceIdentMulti {
	writeNow := false
	for _, opt := range opts {
		writeNow = opt.WriteNow
	}
	return ResourceIdentMulti{
		Provider: provider,
		Purpose:  purpose,
		Type:     object,
		WriteNow: writeNow,
	}
}

// ObjectCache is the main caching provider object. It holds references to some anciliary objects
// as well as a Data structure that is used to hold the K8sResources.
type ObjectCache struct {
	data            map[ResourceIdent]map[types.NamespacedName]*k8sResource
	resourceTracker map[schema.GroupVersionKind]map[types.NamespacedName]bool
	scheme          *runtime.Scheme
	client          client.Client
	ctx             context.Context
	log             logr.Logger
	config          *CacheConfig
}

func NewCacheConfig(scheme *runtime.Scheme, possibleGVKs, protectedGVKs GVKMap, options ...Options) *CacheConfig {
	if possibleGVKs == nil {
		possibleGVKs = make(GVKMap)
	}
	if protectedGVKs == nil {
		protectedGVKs = make(GVKMap)
	}
	var optionObject = Options{}
	if len(options) >= 1 {
		optionObject = options[0]
	}

	if len(optionObject.Ordering) == 0 {
		optionObject.Ordering = []string{
			"*",
			"Deployment",
			"Job",
			"CronJob",
		}
	}

	return &CacheConfig{
		possibleGVKs:  possibleGVKs,
		protectedGVKs: protectedGVKs,
		scheme:        scheme,
		options:       optionObject,
	}
}

type DebugOptions struct {
	Create       bool
	Update       bool
	Apply        bool
	Registration bool
}

type Options struct {
	StrictGVK    bool
	Ordering     []string
	DebugOptions DebugOptions
}

type CacheConfig struct {
	possibleGVKs  GVKMap
	protectedGVKs GVKMap
	scheme        *runtime.Scheme
	options       Options
}

type k8sResource struct {
	Object     client.Object
	Update     utils.Updater
	Status     bool
	jsonData   string
	origObject client.Object
}

type GVKMap map[schema.GroupVersionKind]bool

// NewObjectCache returns an instance of the ObjectCache which defers all applys until the end of
// the reconciliation process, and allows providers to pull objects out of the cache for
// modification.
func NewObjectCache(ctx context.Context, kclient client.Client, logger *logr.Logger, config *CacheConfig) ObjectCache {

	if config.scheme == nil {
		config.scheme = runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(config.scheme))
	}

	if config == nil {
		config = &CacheConfig{}
	}

	var log logr.Logger

	if logger == nil {
		log = logr.Discard()
	} else {
		log = *logger
	}

	return ObjectCache{
		scheme:          config.scheme,
		client:          kclient,
		ctx:             ctx,
		data:            make(map[ResourceIdent]map[types.NamespacedName]*k8sResource),
		resourceTracker: make(map[schema.GroupVersionKind]map[types.NamespacedName]bool),
		log:             log,
		config:          config,
	}
}

func (o *ObjectCache) registerGVK(obj client.Object) {
	gvk, _ := utils.GetKindFromObj(o.scheme, obj)
	if _, ok := o.config.possibleGVKs[gvk]; !ok {
		o.config.possibleGVKs[gvk] = true
		if o.config.options.DebugOptions.Registration {
			fmt.Println("Registered type: ", gvk.Group, gvk.Kind, gvk.Version)
		}
	}
}

// Create first attempts to fetch the object from k8s for initial population. If this fails, the
// blank object is stored in the cache it is imperative that the user of this function call Create
// before modifying the obejct they wish to be placed in the cache.
func (o *ObjectCache) Create(resourceIdent ResourceIdent, nn types.NamespacedName, object client.Object) error {
	if o.config.options.StrictGVK {
		gvk, err := utils.GetKindFromObj(o.scheme, object)
		if err != nil {
			return fmt.Errorf("object type not in schema")
		}
		if _, ok := o.config.possibleGVKs[gvk]; !ok {
			return fmt.Errorf("gvk [%s] of object has not been added to possibleGVKs in config", gvk)
		}
	} else {
		o.registerGVK(object)
	}
	update, err := utils.UpdateOrErr(o.client.Get(o.ctx, nn, object))

	if err != nil {
		return err
	}

	if _, ok := o.data[resourceIdent][nn]; ok {
		return fmt.Errorf("cannot create: ident store [%s] already has item named [%s]", resourceIdent, nn)
	}

	var gvk, obGVK schema.GroupVersionKind
	if gvk, err = utils.GetKindFromObj(o.scheme, resourceIdent.GetType()); err != nil {
		return err
	}

	if obGVK, err = utils.GetKindFromObj(o.scheme, object); err != nil {
		return err
	}

	if gvk != obGVK {
		return fmt.Errorf("create: resourceIdent type does not match runtime object [%s] [%s] [%s]", nn, gvk, obGVK)
	}

	if _, ok := o.resourceTracker[gvk]; !ok {
		o.resourceTracker[gvk] = map[types.NamespacedName]bool{nn: true}
	}

	o.resourceTracker[gvk][nn] = true

	if _, ok := o.data[resourceIdent]; !ok {
		o.data[resourceIdent] = make(map[types.NamespacedName]*k8sResource)
	}

	var jsonData []byte
	if o.config.options.DebugOptions.Create || o.config.options.DebugOptions.Apply {
		jsonData, _ = json.MarshalIndent(object, "", "  ")
	}

	o.data[resourceIdent][nn] = &k8sResource{
		Object:     object.DeepCopyObject().(client.Object),
		Update:     update,
		Status:     false,
		jsonData:   string(jsonData),
		origObject: object.DeepCopyObject().(client.Object),
	}

	if o.config.options.DebugOptions.Create {
		diffVal := "hidden"

		if object.GetObjectKind().GroupVersionKind() != secretCompare {
			diffVal = string(jsonData)
		}

		o.log.Info("CREATE resource ",
			"namespace", nn.Namespace,
			"name", nn.Name,
			"provider", resourceIdent.GetProvider(),
			"purpose", resourceIdent.GetPurpose(),
			"kind", object.GetObjectKind().GroupVersionKind().Kind,
			"diff", diffVal,
		)
	}

	return nil
}

// Update takes the item and tries to update the version in the cache. This will fail if the item is
// not in the cache. A previous provider should have "created" the item before it can be updated.
func (o *ObjectCache) Update(resourceIdent ResourceIdent, object client.Object) error {
	if _, ok := o.data[resourceIdent]; !ok {
		return fmt.Errorf("object cache not found, cannot update")
	}

	nn, err := getNamespacedNameFromRuntime(object)

	if err != nil {
		return err
	}

	if _, ok := o.data[resourceIdent][nn]; !ok {
		return fmt.Errorf("object not found in cache, cannot update")
	}

	var gvk, obGVK schema.GroupVersionKind
	if gvk, err = utils.GetKindFromObj(o.scheme, resourceIdent.GetType()); err != nil {
		return err
	}

	if obGVK, err = utils.GetKindFromObj(o.scheme, object); err != nil {
		return err
	}

	if gvk != obGVK {
		return fmt.Errorf("create: resourceIdent type does not match runtime object [%s] [%s] [%s]", nn, gvk, obGVK)
	}

	o.data[resourceIdent][nn].Object = object.DeepCopyObject().(client.Object)

	if o.config.options.DebugOptions.Update {
		var jsonData []byte
		jsonData, _ = json.MarshalIndent(o.data[resourceIdent][nn].Object, "", "  ")
		if object.GetObjectKind().GroupVersionKind() == secretCompare {
			o.log.Info("UPDATE resource ", "namespace", nn.Namespace, "name", nn.Name, "provider", resourceIdent.GetProvider(), "purpose", resourceIdent.GetPurpose(), "kind", object.GetObjectKind().GroupVersionKind().Kind, "diff", "hidden")
		} else {
			o.log.Info("UPDATE resource ", "namespace", nn.Namespace, "name", nn.Name, "provider", resourceIdent.GetProvider(), "purpose", resourceIdent.GetPurpose(), "kind", object.GetObjectKind().GroupVersionKind().Kind, "diff", string(jsonData))
		}
	}

	if resourceIdent.GetWriteNow() {
		i := o.data[resourceIdent][nn]

		if o.config.options.DebugOptions.Apply {
			jsonData, _ := json.MarshalIndent(i.Object, "", "  ")
			diff := difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(jsonData)),
				B:        difflib.SplitLines(i.jsonData),
				FromFile: "old",
				ToFile:   "new",
				Context:  3,
			}
			text, _ := difflib.GetUnifiedDiffString(diff)
			if i.Object.GetObjectKind().GroupVersionKind() == secretCompare {
				o.log.Info("Update diff", "diff", "hidden", "type", "update", "resType", i.Object.GetObjectKind().GroupVersionKind().Kind, "name", nn.Name, "namespace", nn.Namespace)
			} else {
				o.log.Info("Update diff", "diff", text, "type", "update", "resType", i.Object.GetObjectKind().GroupVersionKind().Kind, "name", nn.Name, "namespace", nn.Namespace)
			}
		}

		if !equality.Semantic.DeepEqual(i.origObject, i.Object) || !bool(i.Update) {
			o.log.Info("INSTANT APPLY resource ", "namespace", nn.Namespace, "name", nn.Name, "provider", resourceIdent.GetProvider(), "purpose", resourceIdent.GetPurpose(), "kind", object.GetObjectKind().GroupVersionKind().Kind, "update", i.Update, "skipped", false)

			if err := i.Update.Apply(o.ctx, o.client, i.Object); err != nil {
				return err
			}
		} else {
			o.log.Info("INSTANT APPLY resource (skipped)", "namespace", nn.Namespace, "name", nn.Name, "provider", resourceIdent.GetProvider(), "purpose", resourceIdent.GetPurpose(), "kind", object.GetObjectKind().GroupVersionKind().Kind, "update", i.Update, "skipped", true)
		}

		if i.Status {
			if err := o.client.Status().Update(o.ctx, i.Object); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *ObjectCache) GetScheme() *runtime.Scheme {
	return o.scheme
}

// Get pulls the item from the cache and populates the given empty object. An error is returned if
// the items are of different types and also if the item is not in the cache. A get should be used
// by a downstream provider. If modifications are made to the object, it should be updated using the
// Update call.
func (o *ObjectCache) Get(resourceIdent ResourceIdent, object client.Object, nn ...types.NamespacedName) error {
	if _, ok := o.data[resourceIdent]; !ok {
		return fmt.Errorf("object cache not found, cannot get")
	}

	if len(nn) > 1 {
		return fmt.Errorf("cannot request more than one named item with get, use list")
	}

	if _, ok := resourceIdent.(ResourceIdentSingle); ok {
		oMap := o.data[resourceIdent]
		for _, v := range oMap {
			if err := o.scheme.Convert(v.Object, object, o.ctx); err != nil {
				return err
			}
			object.GetObjectKind().SetGroupVersionKind(v.Object.GetObjectKind().GroupVersionKind())
		}
	} else {
		v, ok := o.data[resourceIdent][nn[0]]
		if !ok {
			return fmt.Errorf("object not found")
		}
		if err := o.scheme.Convert(v.Object, object, o.ctx); err != nil {
			return err
		}
		object.GetObjectKind().SetGroupVersionKind(v.Object.GetObjectKind().GroupVersionKind())
	}
	return nil
}

// List returns a list of objects stored in the cache for the given ResourceIdent. This list
// behanves like a standard k8s List object although the revision cannot be relied upon. It is
// simply to return something that is familiar to users of k8s client-go.
func (o *ObjectCache) List(resourceIdent ResourceIdentMulti, object runtime.Object) error {
	oMap := o.data[resourceIdent]

	uList := unstructured.UnstructuredList{}

	for _, v := range oMap {
		uobj := unstructured.Unstructured{}
		err := o.scheme.Convert(v.Object, &uobj, o.ctx)
		uobj.SetGroupVersionKind(v.Object.GetObjectKind().GroupVersionKind())
		if err != nil {
			return fmt.Errorf("d: %s", err)
		}
		uList.Items = append(uList.Items, uobj)
	}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(uList.UnstructuredContent(), object)

	if err != nil {
		return err
	}
	return nil
}

// Status marks the object for having a status update
func (o *ObjectCache) Status(resourceIdent ResourceIdent, object client.Object) error {
	if _, ok := o.data[resourceIdent]; !ok {
		return fmt.Errorf("object cache not found, cannot update")
	}

	nn, err := getNamespacedNameFromRuntime(object)

	if err != nil {
		return err
	}

	o.data[resourceIdent][nn].Status = true

	return nil
}

type ObjectToApply struct {
	Ident          ResourceIdent
	NamespacedName types.NamespacedName
	Resource       *k8sResource
}

// Define a collect type that implements the sort.Interface
type objectsToApply struct {
	objs   []ObjectToApply
	scheme *runtime.Scheme
	order  []string
}

func indexOf(elem string, data []string) int {
	for k, v := range data {
		if elem == v {
			return k
		}
	}
	return -1
}

func (u objectsToApply) Len() int {
	return len(u.objs)
}

func (u objectsToApply) Swap(i, j int) {
	u.objs[i], u.objs[j] = u.objs[j], u.objs[i]
}

func (u objectsToApply) Less(i, j int) bool {
	k1 := "*"
	gvk, err := utils.GetKindFromObj(u.scheme, u.objs[i].Ident.GetType())
	if err == nil {
		k1 = gvk.Kind
	}

	k2 := "*"
	gvk, err = utils.GetKindFromObj(u.scheme, u.objs[j].Ident.GetType())
	if err == nil {
		k2 = gvk.Kind
	}
	i1 := indexOf(k1, u.order)
	i2 := indexOf(k2, u.order)
	return i1 < i2
}

// ApplyAll takes all the items in the cache and tries to apply them, given the boolean by the
// update field on the internal resource. If the update is true, then the object will by applied, if
// it is false, then the object will be created.
func (o *ObjectCache) ApplyAll() error {
	dataToSort := objectsToApply{scheme: o.scheme, order: o.config.options.Ordering}
	for res := range o.data {
		for nn := range o.data[res] {
			dataToSort.objs = append(dataToSort.objs, ObjectToApply{
				Ident:          res,
				NamespacedName: nn,
				Resource:       o.data[res][nn],
			})
		}
	}

	sort.Sort(dataToSort)

	err := o.applyResourceCache(dataToSort)
	if err != nil {
		return err
	}

	return nil
}

func (o *ObjectCache) applyResourceCache(cachedData objectsToApply) error {
	for _, v := range cachedData.objs {
		if v.Ident.GetWriteNow() {
			continue
		}
		if o.config.options.DebugOptions.Apply {
			jsonData, _ := json.MarshalIndent(v.Resource.Object, "", "  ")
			diff := difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(jsonData)),
				B:        difflib.SplitLines(v.Resource.jsonData),
				FromFile: "old",
				ToFile:   "new",
				Context:  3,
			}
			text, _ := difflib.GetUnifiedDiffString(diff)
			if v.Resource.Object.GetObjectKind().GroupVersionKind() == secretCompare {
				o.log.Info("Update diff", "diff", "hidden", "type", "update", "resType", v.Resource.Object.GetObjectKind().GroupVersionKind().Kind, "name", v.NamespacedName.Name, "namespace", v.NamespacedName.Namespace)
			} else {
				o.log.Info("Update diff", "diff", text, "type", "update", "resType", v.Resource.Object.GetObjectKind().GroupVersionKind().Kind, "name", v.NamespacedName.Name, "namespace", v.NamespacedName.Namespace)
			}
		}

		if !equality.Semantic.DeepEqual(v.Resource.origObject, v.Resource.Object) || !bool(v.Resource.Update) {
			o.log.Info("APPLY resource ", "namespace", v.NamespacedName.Namespace, "name", v.NamespacedName.Name, "provider", v.Ident.GetProvider(), "purpose", v.Ident.GetPurpose(), "kind", v.Resource.Object.GetObjectKind().GroupVersionKind().Kind, "update", v.Resource.Update, "skipped", false)
			if err := v.Resource.Update.Apply(o.ctx, o.client, v.Resource.Object); err != nil {
				return err
			}
		} else {
			o.log.Info("APPLY resource (skipped)", "namespace", v.NamespacedName.Namespace, "name", v.NamespacedName.Name, "provider", v.Ident.GetProvider(), "purpose", v.Ident.GetPurpose(), "kind", v.Resource.Object.GetObjectKind().GroupVersionKind().Kind, "update", v.Resource.Update, "skipped", true)
		}

		if v.Resource.Status {
			if err := o.client.Status().Update(o.ctx, v.Resource.Object); err != nil {
				return err
			}
		}
	}
	return nil
}

// Debug prints out the contents of the cache.
func (o *ObjectCache) Debug() {
	for iden, v := range o.data {
		fmt.Printf("\n%v-%v", iden.GetProvider(), iden.GetPurpose())
		for pi, i := range v {
			nn, err := getNamespacedNameFromRuntime(i.Object)
			if err != nil {
				fmt.Print(err.Error())
			}
			gvks, _, _ := o.scheme.ObjectKinds(i.Object)
			gvk := gvks[0]
			fmt.Printf("\nObject %v - %v - %v - %v\n", nn, i.Update, gvk, pi)
		}
	}
}

func (o *ObjectCache) AddPossibleGVKFromIdent(objs ...ResourceIdent) {
	for _, obj := range objs {
		gvk, _ := utils.GetKindFromObj(o.scheme, obj.GetType())
		o.config.possibleGVKs[gvk] = true
	}
}

// Reconcile performs the delete on objects that are no longer required
func (o *ObjectCache) Reconcile(ownedUID types.UID, opts ...client.ListOption) error {

	for gvk := range o.config.possibleGVKs {
		if _, ok := o.config.protectedGVKs[gvk]; ok {
			continue
		}
		v, ok := o.resourceTracker[gvk]

		if !ok {
			v = make(map[types.NamespacedName]bool)
		}

		nobjList := unstructured.UnstructuredList{}
		nobjList.SetGroupVersionKind(gvk)

		err := o.client.List(o.ctx, &nobjList, opts...)
		if err != nil {
			return err
		}

		// fmt.Printf("\n%v %v", gvk, len(nobjList.Items))

		for _, obj := range nobjList.Items {
			innerObj := obj
			for _, ownerRef := range innerObj.GetOwnerReferences() {
				if ownerRef.UID == ownedUID {
					nn := types.NamespacedName{
						Name:      innerObj.GetName(),
						Namespace: innerObj.GetNamespace(),
					}
					if err != nil {
						return err
					}
					// fmt.Printf("\n%v\n", v)
					if _, ok := v[nn]; !ok {
						o.log.Info("DELETE resource ", "namespace", innerObj.GetNamespace(), "name", innerObj.GetName(), "kind", innerObj.GetObjectKind().GroupVersionKind().Kind)
						err := o.client.Delete(o.ctx, &innerObj)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	// fmt.Println("\n-----------------")
	return nil
}

func getNamespacedNameFromRuntime(object client.Object) (types.NamespacedName, error) {
	om, err := meta.Accessor(object)

	if err != nil {
		return types.NamespacedName{}, err
	}

	nn := types.NamespacedName{
		Namespace: om.GetNamespace(),
		Name:      om.GetName(),
	}

	return nn, nil
}
