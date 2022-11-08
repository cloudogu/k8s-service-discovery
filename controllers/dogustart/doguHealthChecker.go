package dogustart

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type deploymentReadyChecker struct {
	namespace string
	client    kubernetes.Interface
}

// NewDeploymentReadyChecker creates a new instance of a health checker capable of checking whether a deployment has a currently ready pod.
func NewDeploymentReadyChecker(clientset kubernetes.Interface, namespace string) *deploymentReadyChecker {
	return &deploymentReadyChecker{
		namespace: namespace,
		client:    clientset,
	}
}

// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
func (d *deploymentReadyChecker) IsReady(ctx context.Context, deploymentName string) (bool, error) {
	deployment, err := d.client.AppsV1().Deployments(d.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	log.FromContext(ctx).Info(fmt.Sprintf("Found deployment for jekins with values: [%+v]", deployment))
	return deployment.Status.ReadyReplicas > 0, nil
}

// WaitForReady allows the execution of code when the deployment switches from the not ready state into the ready state.
func (d *deploymentReadyChecker) WaitForReady(ctx context.Context, deploymentName string, onReady func(ctx context.Context)) error {
	ok, err := d.IsReady(ctx, deploymentName)
	if err != nil {
		return err
	}

	log.FromContext(ctx).Info(fmt.Sprintf("is ready? %t", ok))

	if ok {
		// pod is already ready
		onReady(ctx)
		return nil
	}

	watchOptions := metav1.ListOptions{}
	watchOptions.FieldSelector = fmt.Sprintf("metadata.name=%s", deploymentName)
	watch, err := d.client.AppsV1().Deployments(d.namespace).Watch(ctx, watchOptions)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			watch.Stop()
			return nil
		case <-watch.ResultChan():
			ok, err := d.IsReady(ctx, deploymentName)
			if err != nil {
				return err
			}

			if ok {
				onReady(ctx)
				watch.Stop()
				return nil
			}
		default:
			time.Sleep(3 * time.Second)
		}
	}
}
