#!/bin/bash
# Example: Deploy web application across multiple agents with verification

set -e

# Configuration
LOCAL_IMAGE="myapp:latest"
REMOTE_IMAGE="myapp:production"
AGENTS=("agent-1" "agent-2" "agent-3")
CONTAINER_NAME="myapp"
PORT="8080:8080"
ENV_VARS=(
  "ENVIRONMENT=production"
  "LOG_LEVEL=info"
  "DB_HOST=db.prod.internal"
)

echo "🚀 Multi-Agent Deployment Pipeline"
echo "=================================="
echo "Local Image:  $LOCAL_IMAGE"
echo "Remote Image: $REMOTE_IMAGE"
echo "Target Agents: ${AGENTS[@]}"
echo ""

# Function to deploy to an agent
deploy_agent() {
  local agent=$1
  echo ""
  echo "📦 Deploying to $agent..."
  
  # Build the command
  cmd="mandau deploy container $LOCAL_IMAGE $REMOTE_IMAGE --agent $agent --up-remote --verify --name $CONTAINER_NAME-$agent -p $PORT"
  
  for env in "${ENV_VARS[@]}"; do
    cmd="$cmd -e $env"
  done
  
  # Execute with dry-run first for verification
  echo "   Preview:"
  eval "$cmd --dry-run"
  
  read -p "   Proceed? (y/n) " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "   ⏭️  Skipped"
    return 1
  fi
  
  # Execute actual deployment
  if eval "$cmd"; then
    echo "   ✅ Deployment successful"
    return 0
  else
    echo "   ❌ Deployment failed"
    return 1
  fi
}

# Deploy to all agents
failed_agents=()
for agent in "${AGENTS[@]}"; do
  if ! deploy_agent "$agent"; then
    failed_agents+=("$agent")
  fi
done

echo ""
echo "📊 Deployment Summary"
echo "===================="
echo "Total agents: ${#AGENTS[@]}"
echo "Failed: ${#failed_agents[@]}"

if [ ${#failed_agents[@]} -eq 0 ]; then
  echo "✅ All agents deployed successfully"
  exit 0
else
  echo "⚠️  Failed agents: ${failed_agents[@]}"
  exit 1
fi
