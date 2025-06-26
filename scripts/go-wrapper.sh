#!/bin/bash

# This script serves as a wrapper for go commands to override the toolchain settings

# Set environment variables to disable toolchain management
export GOTOOLCHAIN=local
export GO111MODULE=on

# Execute the go command with all the original arguments
go "$@"
