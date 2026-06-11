package ImagePolicy

import (
	"testing"

	imagePolicyModel "private_browser_server/Models/ImagePolicy"
	nodeModel "private_browser_server/Models/Node"
)

func TestResolveRuntimeImageStableAMD64(t *testing.T) {
	image, err := ResolveRuntimeImage(imagePolicyModel.ImageChannelStable, nodeModel.NodeArchAMD64)
	if err != nil {
		t.Fatalf("ResolveRuntimeImage returned error: %v", err)
	}
	if image != defaultStableAMD64Image {
		t.Fatalf("expected amd64 image %s, got %s", defaultStableAMD64Image, image)
	}
}

func TestResolveRuntimeImageStableARM64(t *testing.T) {
	image, err := ResolveRuntimeImage("", nodeModel.NodeArchARM64)
	if err != nil {
		t.Fatalf("ResolveRuntimeImage returned error: %v", err)
	}
	if image != defaultStableARM64Image {
		t.Fatalf("expected arm64 image %s, got %s", defaultStableARM64Image, image)
	}
}

func TestResolveRuntimeImagePlatformValueAMD64(t *testing.T) {
	image, err := ResolveRuntimeImage(defaultStableAMD64Image, nodeModel.NodeArchAMD64)
	if err != nil {
		t.Fatalf("ResolveRuntimeImage returned error: %v", err)
	}
	if image != defaultStableAMD64Image {
		t.Fatalf("expected amd64 image %s, got %s", defaultStableAMD64Image, image)
	}
}

func TestResolveRuntimeImageRejectsPlatformValueArchMismatch(t *testing.T) {
	if _, err := ResolveRuntimeImage(defaultStableAMD64Image, nodeModel.NodeArchARM64); err == nil {
		t.Fatalf("expected amd64 platform value to fail on arm64 node")
	}
}

func TestResolveRuntimeImageRejectsUnknownArch(t *testing.T) {
	if _, err := ResolveRuntimeImage(imagePolicyModel.ImageChannelStable, nodeModel.NodeArchUnknown); err == nil {
		t.Fatalf("expected unknown arch to fail")
	}
}

func TestResolveRuntimeImageRejectsUnknownPolicyValue(t *testing.T) {
	if _, err := ResolveRuntimeImage("docker.io/example/custom:latest", nodeModel.NodeArchAMD64); err == nil {
		t.Fatalf("expected unknown policy value to fail")
	}
}
