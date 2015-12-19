#!/bin/bash

gotestcover -v -covermode=count -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
