#!/bin/bash

# Script to update dependencies in the project

set -e

# Get specific missing dependencies first
echo "Getting specific dependencies..."
go get github.com/valyala/fasthttp@v1.50.0
go get sigs.k8s.io/controller-runtime/pkg/envtest@v0.21.0
go get sigs.k8s.io/controller-runtime/pkg/metrics@v0.21.0
go get sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue@v0.21.0
go get sigs.k8s.io/controller-runtime/pkg/client/fake@v0.21.0
go get github.com/google/cel-go/parser@v0.23.2
go get google.golang.org/genproto/googleapis/api/expr/v1alpha1@v0.0.0-20241209162323-e6fa225c2576
go get cuelang.org/go/mod/modconfig@v0.8.0
go get sigs.k8s.io/controller-runtime/pkg/leaderelection@v0.21.0
go get k8s.io/cli-runtime/pkg/genericclioptions@v0.29.14
go get k8s.io/cli-runtime/pkg/resource@v0.29.14
go get k8s.io/kubectl/pkg/util/deployment@v0.29.0

# Update Go modules
echo "Updating Go modules..."
go mod tidy

# Remove the controller-runtime vendor directory if it exists
if [ -d "vendor/sigs.k8s.io/controller-runtime" ]; then
    echo "Removing vendored controller-runtime..."
    rm -rf vendor/sigs.k8s.io/controller-runtime
fi

# Force-download direct and indirect dependencies to update go.sum
echo "Force-downloading dependencies..."
go mod download all

# Verify dependency resolution
echo "Verifying dependencies..."
go mod verify

echo "Done updating dependencies!"
