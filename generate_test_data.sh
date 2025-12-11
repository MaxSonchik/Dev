#!/bin/bash
TARGET="/tmp/devos-graph-test"
rm -rf $TARGET
mkdir -p $TARGET
cd $TARGET

echo "🚀 Generating Git Graph Divergence..."

# Init
git init -q
git config user.email "bot@devos.io"
git config user.name "DevOS Bot"

# 1. Base
touch README.md
git add .
git commit -m "init: base project" -q

# 2. Create feature branch
git checkout -b feature/login -q
touch login.go
git add .
git commit -m "feat: add login logic" -q

# 3. CRITICAL STEP: Switch back to master and commit SOMETHING ELSE
# This forces the histories to diverge (Y-shape)
git checkout master -q
touch hotfix.txt
git add .
git commit -m "fix: critical hotfix on prod" -q

# 4. Now merge - this creates the bubble
git merge feature/login --no-ff -m "merge: feature login" -q

# 5. Add more files for status check
echo "changes" >> README.md
touch new_untracked_file.txt
mkdir infra
touch infra/k8s.yaml

# 6. Add fake Docker/Infra for d-env to detect stacks
echo "FROM alpine" > Dockerfile
echo "go 1.21" > go.mod

echo "✅ Graph Data Ready at $TARGET"