package utils

import (
	"context"
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"math/big"
	mrand "math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const pCharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!\"#$%&'()*+,-./:;<>=?@^~"
const rCharSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const lCharSet = "abcdefghijklmnopqrstuvwxyz0123456789"

func init() {
	mrand.Seed(time.Now().UnixNano())
}

// Log is a null logger instance.
var Log logr.Logger = logr.Discard()

func buildRandString(n int, charset string) string {
	b := make([]byte, n)

	for i := range b {
		b[i] = charset[mrand.Intn(len(charset))]
	}

	return string(b)
}

// RandString generates a random string of length n
func RandPassword(n int) (string, error) {
	if n < 14 {
		return "", fmt.Errorf("random password does not meet complexity guidelines must be more than 14 chars")
	}

	b := make([]byte, n)

	max := big.NewInt(int64(len(pCharSet)))
	for i := range b {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = pCharSet[num.Int64()]
	}

	return string(b), nil
}

// RandString generates a random string of length n
func RandString(n int) string {
	return buildRandString(n, rCharSet)
}

// RandStringLower generates a random string of length n
func RandStringLower(n int) string {
	return buildRandString(n, lCharSet)
}

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Updater is a bool type object with functions attached that control when a resource should be
// created or applied.
type Updater bool

// Apply will apply the resource if it already exists, and create it if it does not. This is based
// on the bool value of the Update object.
func (u *Updater) Apply(ctx context.Context, cl client.Client, obj client.Object) error {
	var err error
	var kind string

	if obj.GetObjectKind().GroupVersionKind().Kind == "" {
		kind = reflect.TypeOf(obj).String()
	} else {
		kind = obj.GetObjectKind().GroupVersionKind().Kind
	}

	meta := obj.(metav1.Object)

	if *u {
		//Log.Info("Updating resource", "namespace", meta.GetNamespace(), "name", meta.GetName(), "kind", kind)
		err = cl.Update(ctx, obj)
	} else {
		if meta.GetName() == "" {
			return nil
		}

		//Log.Info("Creating resource", "namespace", meta.GetNamespace(), "name", meta.GetName(), "kind", kind)
		err = cl.Create(ctx, obj)
	}

	if err != nil {
		verb := "creating"
		if *u {
			verb = "updating"
		}

		return fmt.Errorf("error %s resource %s %s: %s", verb, kind, meta.GetName(), err.Error())
	}

	return nil
}

// UpdateOrErr returns an update object if the err supplied is nil.
func UpdateOrErr(err error) (Updater, error) {
	update := Updater(err == nil)

	if err != nil && !k8serr.IsNotFound(err) {
		return update, err
	}

	return update, nil
}

// UpdateAllOrErr queries the client for a range of objects and returns updater objects for each.
func UpdateAllOrErr(ctx context.Context, cl client.Client, nn types.NamespacedName, obj ...client.Object) (map[client.Object]Updater, error) {
	updates := map[client.Object]Updater{}

	for _, resource := range obj {
		update, err := UpdateOrErr(cl.Get(ctx, nn, resource))

		if err != nil {
			return updates, err
		}

		updates[resource] = update
	}

	return updates, nil
}

// ApplyAll applies all the update objects in the list called updates.
func ApplyAll(ctx context.Context, cl client.Client, updates map[client.Object]Updater) error {
	for resource, update := range updates {
		if err := update.Apply(ctx, cl, resource); err != nil {
			return err
		}
	}

	return nil
}

// B64Decode decodes the provided secret
func B64Decode(s *core.Secret, key string) (string, error) {
	decoded, err := b64.StdEncoding.DecodeString(string(s.Data[key]))

	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func intMinMax(listStrInts []string, max bool) (string, error) {
	var listInts []int
	for _, strint := range listStrInts {
		i, err := strconv.Atoi(strint)
		if err != nil {
			return "", err
		}
		listInts = append(listInts, i)
	}
	ol := listInts[0]
	for i, e := range listInts {
		if max {
			if i == 0 || e > ol {
				ol = e
			}
		} else {
			if i == 0 || e < ol {
				ol = e
			}
		}
	}
	return strconv.Itoa(ol), nil
}

// IntMin takes a list of integers as strings and returns the minimum.
func IntMin(listStrInts []string) (string, error) {
	return intMinMax(listStrInts, false)
}

// IntMax takes a list of integers as strings and returns the maximum.
func IntMax(listStrInts []string) (string, error) {
	return intMinMax(listStrInts, true)
}

// ListMerge takes a list comma separated strings and performs a set union on them.
func ListMerge(listStrs []string) (string, error) {
	optionStrings := make(map[string]bool)
	for _, optionsList := range listStrs {
		brokenString := strings.Split(optionsList, ",")
		for _, option := range brokenString {
			optionStrings[strings.TrimSpace(option)] = true
		}
	}
	keys := make([]string, len(optionStrings))

	i := 0
	for key := range optionStrings {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return strings.Join(keys, ","), nil
}

func MakeOwnerReference(i client.Object) metav1.OwnerReference {
	ovk := i.GetObjectKind().GroupVersionKind()
	return metav1.OwnerReference{
		APIVersion: ovk.GroupVersion().String(),
		Kind:       ovk.Kind,
		Name:       i.GetName(),
		UID:        i.GetUID(),
		Controller: TruePtr(),
	}

}

// MakeLabeler creates a function that will label objects with metadata from
// the given namespaced name and labels
func MakeLabeler(nn types.NamespacedName, labels map[string]string, obj client.Object) func(metav1.Object) {
	return func(o metav1.Object) {
		o.SetName(nn.Name)
		o.SetNamespace(nn.Namespace)
		o.SetLabels(labels)
		o.SetOwnerReferences([]metav1.OwnerReference{MakeOwnerReference(obj)})
	}
}

// // GetCustomLabeler takes a set of labels and returns a labeler function that
// // will apply those labels to a reource.
func GetCustomLabeler(labels map[string]string, nn types.NamespacedName, baseResource client.Object) func(metav1.Object) {
	appliedLabels := baseResource.GetLabels()
	for k, v := range labels {
		appliedLabels[k] = v
	}
	return MakeLabeler(nn, appliedLabels, baseResource)
}

// // MakeService takes a service object and applies the correct ownership and labels to it.
func MakeService(service *core.Service, nn types.NamespacedName, labels map[string]string, ports []core.ServicePort, baseResource client.Object, nodePort bool) {
	labeler := GetCustomLabeler(labels, nn, baseResource)
	labeler(service)
	service.Spec.Selector = labels
	if nodePort {
		for i, sport := range ports {
			for _, dport := range service.Spec.Ports {
				if sport.Name == dport.Name {
					if dport.NodePort != 0 {
						sport.NodePort = dport.NodePort
					}
					break
				}
			}
			ports[i] = sport
		}
		service.Spec.Type = "NodePort"
	} else {
		service.Spec.Type = "ClusterIP"
	}
	service.Spec.Ports = ports
}

// // MakePVC takes a PVC object and applies the correct ownership and labels to it.
func MakePVC(pvc *core.PersistentVolumeClaim, nn types.NamespacedName, labels map[string]string, size string, baseResource client.Object) {
	labeler := GetCustomLabeler(labels, nn, baseResource)
	labeler(pvc)
	pvc.Spec.AccessModes = []core.PersistentVolumeAccessMode{core.ReadWriteOnce}
	pvc.Spec.Resources = core.ResourceRequirements{
		Requests: core.ResourceList{
			core.ResourceName(core.ResourceStorage): resource.MustParse(size),
		},
	}
}

// IntPtr returns a pointer to the passed integer.
func IntPtr(i int) *int {
	return &i
}

// BoolPtr returns a pointer to the passed boolean.
func BoolPtr(b bool) *bool {
	boolVar := b
	return &boolVar
}

// GetKindFromObj retrieves GVK associated with registered runtime.Object
func GetKindFromObj(scheme *runtime.Scheme, object runtime.Object) (schema.GroupVersionKind, error) {
	gvks, nok, err := scheme.ObjectKinds(object)

	if err != nil {
		return schema.EmptyObjectKind.GroupVersionKind(), err
	}

	if nok {
		return schema.EmptyObjectKind.GroupVersionKind(), fmt.Errorf("object type is unknown")
	}

	return gvks[0], nil
}

// CopySecret will return a *core.Secret that is copied from a source NamespaceName and intended to
// be applied into a destination NamespacedName
func CopySecret(ctx context.Context, client client.Client, srcSecretRef types.NamespacedName, dstSecretRef types.NamespacedName) (*core.Secret, error) {
	nullRef := types.NamespacedName{}
	if srcSecretRef == nullRef {
		return nil, fmt.Errorf("srcSecretRef is an empty NamespacedName")
	}
	if dstSecretRef == nullRef {
		return nil, fmt.Errorf("dstSecretRef is an empty NamespacedName")
	}

	srcSecret := &core.Secret{}

	if err := client.Get(ctx, srcSecretRef, srcSecret); err != nil {
		return nil, err
	}

	newSecret := &core.Secret{}
	newSecret.Immutable = srcSecret.Immutable
	newSecret.Data = srcSecret.Data
	newSecret.Type = srcSecret.Type
	newSecret.SetName(dstSecretRef.Name)
	newSecret.SetNamespace(dstSecretRef.Namespace)

	return newSecret, nil
}

// Int32Ptr returns a pointer to an int32 version of n
func Int32Ptr(n int) *int32 {
	t, err := Int32(n)
	if err != nil {
		panic(err)
	}
	return &t
}

// Int64Ptr returns a pointer to an int64
func Int64Ptr(n int64) *int64 {
	return &n
}

// TruePtr returns a pointer to True
func TruePtr() *bool {
	t := true
	return &t
}

// FalsePtr returns a pointer to True
func FalsePtr() *bool {
	f := false
	return &f
}

// StringPtr returns a pointer to True
func StringPtr(str string) *string {
	s := str
	return &s
}

type Annotator interface {
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)
}

func UpdateAnnotations(obj Annotator, desiredAnnotations ...map[string]string) {
	annotations := obj.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	for _, annotationsSource := range desiredAnnotations {
		for k, v := range annotationsSource {
			annotations[k] = v
		}
	}
	obj.SetAnnotations(annotations)
}
