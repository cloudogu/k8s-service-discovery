package warp

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	coreosclient "github.com/coreos/etcd/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Order can be used to modify ordering via configuration
type Order map[string]int

// Configuration for warp menu creation
type Configuration struct {
	Sources []Source
	Target  string
	Order   Order
	Support []SupportSource
}

// Source in etcd
type Source struct {
	Path string
	Type string
	Tag  string
}

// SupportSource for SupportEntries from yaml
type SupportSource struct {
	Identifier string
	External   bool
	Href       string
}

// Run creates the warp menu and update the menu whenever a relevant etcd key was changed
func Run(configuration *Configuration, registry registry.WatchConfigurationContext, k8sClient client.Client) {
	log.Println("start watcher for warp entries")
	warpChannel := make(chan *coreosclient.Response)
	for _, source := range configuration.Sources {
		go func(source Source) {
			for {
				execute(configuration, registry, k8sClient)
				registry.Watch(source.Path, true, warpChannel)
			}
		}(source)
	}
	for range warpChannel {
		execute(configuration, registry, k8sClient)
	}
}

func execute(configuration *Configuration, registry registry.WatchConfigurationContext, k8sClient client.Client) {
	reader := ConfigReader{
		registry:      registry,
		configuration: configuration,
	}
	categories, err := reader.readFromConfig(configuration)
	if err != nil {
		log.Println("Error during read:", err)
		return
	}
	log.Printf("all found Categories: %v", categories)
	err = jsonWriter(categories, k8sClient)
	if err != nil {
		log.Printf("failed to write warp menu as json: %v", err)
	}
}

// JSONWriter converts the data to a json
func jsonWriter(data interface{}, client client.Client) error {
	configmap, err := getMenuConfigMap(client)
	if err != nil {
		return fmt.Errorf("failed to get menu json config map: %w", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal warp data: %w", err)
	}

	configmap.Data["menu.json"] = string(jsonData)

	err = client.Update(context.TODO(), configmap)
	if err != nil {
		return fmt.Errorf("failed to update menu json config map: %w", err)
	}

	return nil
}

func getMenuConfigMap(k8sClient client.Client) (*corev1.ConfigMap, error) {
	configmap := &corev1.ConfigMap{}
	objectKey := client.ObjectKey{Name: "menu-json", Namespace: "ecosystem"}
	err := k8sClient.Get(context.Background(), objectKey, configmap)

	return configmap, err
}
