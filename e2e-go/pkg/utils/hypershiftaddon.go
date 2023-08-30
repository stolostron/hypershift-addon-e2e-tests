package utils

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsHypershiftOperatorHealthy checks if the operator and external-dns operators are in healthy state
func IsHypershiftOperatorHealthy(kubeClient kubernetes.Interface) error {
	deployment, err :=
		kubeClient.AppsV1().Deployments(HypershiftOperatorNamespace).Get(context.TODO(), HypershiftOperatorName, metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		return err
	}

	dnsDeployment, err :=
		kubeClient.AppsV1().Deployments(HypershiftOperatorNamespace).Get(context.TODO(), HyperShiftDNSOperatorName, metav1.GetOptions{})
	if err != nil {
		fmt.Println(err)
		return err
	}

	if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas ||
		dnsDeployment.Status.AvailableReplicas != *dnsDeployment.Spec.Replicas {
		return errors.New("ERROR: Hypershift operator is not healthy. Available replicas is not equal to the number of replicas")
	} else {
		fmt.Printf("Hypershift operator is healthy with %d Available replicas for %s!\n", deployment.Status.AvailableReplicas, HypershiftOperatorName)
		fmt.Printf("Hypershift operator is healthy with %d Available replicas for %s!\n", deployment.Status.AvailableReplicas, HyperShiftDNSOperatorName)
	}

	return err
}
