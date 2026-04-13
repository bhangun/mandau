#!/bin/bash
# Example: Blue-green deployment strategy with traffic switchover

set -e

LOCAL_IMAGE="${1:-myapp:latest}"
CURRENT_VERSION="${2:-blue}"
NEW_VERSION="${3:-green}"

BLUE_PORT="8080:8080"
GREEN_PORT="8081:8081"

echo "🔵🟢 Blue-Green Deployment"
echo "=========================="
echo "Image:           $LOCAL_IMAGE"
echo "Current (Blue):  :$BLUE_PORT"
echo "New (Green):     :$GREEN_PORT"
echo ""

# Deploy green version
echo "📦 Deploying GREEN version..."
mandau deploy container "$LOCAL_IMAGE" "myapp:green" \
  --up-remote \
  --name myapp-green \
  --port "$GREEN_PORT" \
  --env VERSION=green \
  --verify

echo ""
echo "🧪 GREEN is running on port 8081"
echo "   Test with: curl http://localhost:8081/health"
echo ""

read -p "Is GREEN healthy? Proceed with switchover? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "❌ Switchover cancelled. GREEN remains running."
  echo "   Stop GREEN with: mandau docker stop myapp-green"
  exit 1
fi

# Stop blue, rename green to blue
echo ""
echo "🔄 Switching traffic..."
mandau docker stop myapp-blue || true
mandau docker rm myapp-blue || true

mandau docker rename myapp-green myapp-blue
mandau docker update --restart=always myapp-blue

echo ""
echo "✅ Switchover complete!"
echo "   BLUE (production) is now running on port 8080"
echo ""
echo "📝 To rollback:"
echo "   mandau deploy container myapp:blue myapp:blue --up-remote --name myapp-blue -p 8080:8080"
