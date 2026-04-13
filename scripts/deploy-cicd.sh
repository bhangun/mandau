#!/bin/bash
# Example: CI/CD integration - Deploy from Git commit

set -e

# Build image
echo "🔨 Building Docker image..."
docker build -t myapp:$GIT_COMMIT .

# Tag for release
docker tag myapp:$GIT_COMMIT myapp:latest

echo "✅ Build complete: myapp:$GIT_COMMIT"
echo ""

# Determine target environment
if [[ "$GIT_BRANCH" == "main" ]]; then
  ENV="production"
  AGENT="prod-1"
  PORTS=("8080:8080")
elif [[ "$GIT_BRANCH" == "develop" ]]; then
  ENV="staging"
  AGENT="staging-1"
  PORTS=("9000:9000")
else
  echo "⏭️  Skipping deployment for branch: $GIT_BRANCH"
  exit 0
fi

echo "🚀 Deploying to $ENV..."
mandau deploy container "myapp:latest" "myapp:$ENV" \
  --agent "$AGENT" \
  --up-remote \
  --verify \
  --name "myapp-$ENV" \
  --port "${PORTS[0]}" \
  --env "ENVIRONMENT=$ENV" \
  --env "GIT_COMMIT=$GIT_COMMIT" \
  --env "GIT_BRANCH=$GIT_BRANCH" \
  --env "BUILD_ID=$CI_BUILD_ID"

echo ""
echo "✅ Deployment successful!"
echo "   Environment: $ENV"
echo "   Agent: $AGENT"
echo "   Image: myapp:$ENV"
echo "   Git Commit: $GIT_COMMIT"
