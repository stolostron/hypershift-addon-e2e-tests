package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var ConsoleCLIDownloadGVR = schema.GroupVersionResource{
	Group:    "console.openshift.io",
	Version:  "v1",
	Resource: "consoleclidownloads",
}

func GetHCPConsoleCliDownload(hubClientDynamic dynamic.Interface) (*unstructured.Unstructured, error) {
	fmt.Printf("Getting the HCP ConsoleCLIDownload resource %s\n", HCPCliDownloadName)
	hcpConsoleDownload, err := GetConsoleCliDownload(hubClientDynamic, HCPCliDownloadName)
	if err != nil {
		return nil, err
	}
	return hcpConsoleDownload, err
}

func GetConsoleCliDownload(hubClientDynamic dynamic.Interface, name string) (*unstructured.Unstructured, error) {
	fmt.Printf("Getting the ConsoleCLIDownload resource %s\n", name)
	consoleDownload, err := GetResource(hubClientDynamic, ConsoleCLIDownloadGVR, "", name)
	if err != nil {
		return nil, err
	}
	return consoleDownload, err
}
