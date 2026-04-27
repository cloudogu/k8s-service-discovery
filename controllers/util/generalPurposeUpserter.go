package util

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type dynamicResourceClient interface {
	List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error
	Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
}

type GeneralPurposeUpserter struct {
	resourceClient dynamicResourceClient
	// TODO improve by using field and label selectors
	listOptions metav1.ListOptions
}

func NewGeneralPurposeUpserter(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace string, listOptions metav1.ListOptions) *GeneralPurposeUpserter {
	return &GeneralPurposeUpserter{
		resourceClient: dynamicClient.Resource(gvr).Namespace(namespace),
		listOptions:    listOptions,
	}
}

// Upsert will update or create the objects in the map.
// It will delete any existing objects that are not in the map and match the specified list options.
// If the map only contains objects which match the list options, this method is fully declarative.
// TODO add validation of the objects that get passed in
func (u *GeneralPurposeUpserter) Upsert(ctx context.Context, objects map[string]unstructured.Unstructured) error {
	existingObjects, err := u.resourceClient.List(ctx, u.listOptions)
	if err != nil {
		return err
	}

	var updateErrs []error
	var deleteErrs []error
	for _, existingObject := range existingObjects.Items {
		if _, ok := objects[existingObject.GetName()]; ok {
			_, err := u.resourceClient.Update(ctx, &existingObject, metav1.UpdateOptions{})
			if err != nil {
				updateErrs = append(updateErrs, err)
			}

			delete(objects, existingObject.GetName())
		} else {
			err := u.resourceClient.Delete(ctx, existingObject.GetName(), metav1.DeleteOptions{})
			if err != nil {
				deleteErrs = append(deleteErrs, err)
			}
		}
	}

	var updateErr error
	if len(updateErrs) > 0 {
		updateErr = fmt.Errorf("failed to update some objects: %w", errors.Join(updateErrs...))
	}

	var deleteErr error
	if len(deleteErrs) > 0 {
		deleteErr = fmt.Errorf("failed to delete some objects: %w", errors.Join(deleteErrs...))
	}

	var createErrs []error
	for _, object := range objects {
		_, err := u.resourceClient.Create(ctx, &object, metav1.CreateOptions{})
		if err != nil {
			createErrs = append(createErrs, err)
		}
	}

	var createErr error
	if len(createErrs) > 0 {
		createErr = fmt.Errorf("failed to create some objects: %w", errors.Join(createErrs...))
	}

	return errors.Join(updateErr, deleteErr, createErr)
}
