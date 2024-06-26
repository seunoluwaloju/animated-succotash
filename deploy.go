package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

func main() {
	var kubeConfig *string
	if home := homeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "path to kube config file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "path to the kube config file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Get all pods in the cluster...")
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Iterate through each pod and redeploy if the name contains "database"
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, "database") {
			fmt.Printf("Redeploying pod %s in namespace %s\n", pod.Name, pod.Namespace)

			// Perform a graceful restart using annotation approach
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				// Retrieve the latest pod object
				currentPod, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get pod %s: %v", pod.Name, err)
				}

				// trigger pod restart
				annotations := currentPod.Annotations
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

				currentPod.Annotations = annotations
				_, updateErr := clientset.CoreV1().Pods(pod.Namespace).Update(context.Background(), currentPod, metav1.UpdateOptions{})
				return updateErr
			})
			if err != nil {
				fmt.Printf("Failed to redeploy pod %s in namespace %s: %v\n", pod.Name, pod.Namespace, err)
			} else {
				fmt.Printf("Pod %s in namespace %s redeployed successfully\n", pod.Name, pod.Namespace)
			}
		} else {
			fmt.Printf("No Pod in namespace %s contains 'database' in its name\n", pod.Namespace)
		}
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}
