#!/bin/bash
TARGET="/tmp/devos-infra-test"
rm -rf $TARGET
mkdir -p $TARGET
cd $TARGET

echo "🚀 Generating Infrastructure Test Data..."

# 1. Project Init
git init -q
echo "# Enterprise Infrastructure" > README.md

# 2. Terraform (AWS)
mkdir -p infra/terraform/vpc
cat <<EOF > infra/terraform/vpc/main.tf
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
resource "aws_subnet" "public" {
  vpc_id = aws_vpc.main.id
}
EOF

# 3. Kubernetes (Manifests)
mkdir -p infra/k8s/apps
cat <<EOF > infra/k8s/apps/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-service
  labels:
    app: payment
EOF

cat <<EOF > infra/k8s/apps/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: payment-lb
EOF

# 4. GitHub Actions (Release)
mkdir -p .github/workflows
cat <<EOF > .github/workflows/release.yml
name: Release
jobs:
  build-and-release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make build
EOF

# 5. Stacks
echo "go 1.21" > go.mod
echo "python" > requirements.txt

echo "✅ Data generated at $TARGET"
