package util

import (
	"context"
	"errors"
	"fmt"
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type dynamicResourceClient interface {
	List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error
	Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
}

type DeclarativeUpserter struct {
	resourceClient dynamicResourceClient
	gvr            schema.GroupVersionResource
}

func NewDeclarativeUpserter(
	dynamicClient dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
) *DeclarativeUpserter {
	return &DeclarativeUpserter{
		resourceClient: dynamicClient.Resource(gvr).Namespace(namespace),
		gvr:            gvr,
	}
}

// Upsert will update or create the objects in the map.
// It will delete any existing objects that are not in the map and match the specified label selector.
// The objects must match the label selector, so this method is fully declarative.
func (u *DeclarativeUpserter) Upsert(ctx context.Context, labelSelector labels.Selector, objects map[string]unstructured.Unstructured) error {
	// validation
	var validationErrs []error
	for _, object := range objects {
		if !labelSelector.Matches(labels.Set(object.GetLabels())) {
			validationErrs = append(validationErrs, fmt.Errorf("%s [%s] does not match label selector [%s]", u.gvr, object.GetName(), labelSelector))
		}
	}
	if len(validationErrs) > 0 {
		return fmt.Errorf("failed to upsert %s: %w", u.gvr, errors.Join(validationErrs...))
	}

	existingObjects, err := u.resourceClient.List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}

	remainingObjects, updateErr, deleteErr := u.updateOrDelete(ctx, existingObjects, objects)
	createErr := u.create(ctx, remainingObjects)

	return errors.Join(updateErr, deleteErr, createErr)
}

func (u *DeclarativeUpserter) create(ctx context.Context, objects map[string]unstructured.Unstructured) error {
	var createErrs []error
	for _, object := range objects {
		_, err := u.resourceClient.Create(ctx, &object, metav1.CreateOptions{})
		if err != nil {
			createErrs = append(createErrs, err)
		}
	}

	var createErr error
	if len(createErrs) > 0 {
		createErr = fmt.Errorf("failed to create %s: %w", u.gvr, errors.Join(createErrs...))
	}

	return createErr
}

func (u *DeclarativeUpserter) updateOrDelete(ctx context.Context, existingObjects *unstructured.UnstructuredList, objects map[string]unstructured.Unstructured) (remainingObjects map[string]unstructured.Unstructured, updateErr error, deleteErr error) {
	// clone to avoid side effects
	remainingObjects = maps.Clone(objects)
	var updateErrs []error
	var deleteErrs []error
	for _, existingObject := range existingObjects.Items {
		if _, ok := objects[existingObject.GetName()]; ok {
			_, err := u.resourceClient.Update(ctx, &existingObject, metav1.UpdateOptions{})
			if err != nil {
				updateErrs = append(updateErrs, err)
			}

			delete(remainingObjects, existingObject.GetName())
		} else {
			err := u.resourceClient.Delete(ctx, existingObject.GetName(), metav1.DeleteOptions{})
			if err != nil {
				deleteErrs = append(deleteErrs, err)
			}
		}
	}

	if len(updateErrs) > 0 {
		updateErr = fmt.Errorf("failed to update %s: %w", u.gvr, errors.Join(updateErrs...))
	}

	if len(deleteErrs) > 0 {
		deleteErr = fmt.Errorf("failed to delete %s: %w", u.gvr, errors.Join(deleteErrs...))
	}

	return remainingObjects, updateErr, deleteErr
}
