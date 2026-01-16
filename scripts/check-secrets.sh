#!/usr/bin/env bash
# Check which secrets are configured

echo "Checking Daisy environment configuration..."
echo ""

if [ ! -f ".env.sh" ]; then
    echo "❌ .env.sh not found in workspace"
    echo "   Copy template: cp \$DAISY_ROOT/templates/env.sh.template .env.sh"
    exit 1
fi

source .env.sh

check_secret() {
    local name=$1
    local var=$2
    if [ -z "${!var}" ]; then
        echo "⚠️  $name: Not configured"
    else
        # Show first 8 chars only
        local preview="${!var:0:8}..."
        echo "✅ $name: $preview"
    fi
}

echo "Core paths:"
check_secret "DAISY_ROOT" "DAISY_ROOT"
check_secret "DAISY_HOME" "DAISY_HOME"

echo ""
echo "API tokens:"
check_secret "Webex API" "DAISY_SECRET_WEBEX_API_TOKEN"
check_secret "Backstage" "DAISY_SECRET_BACKSTAGE_TOKEN"
check_secret "JIRA API" "DAISY_SECRET_JIRA_API_TOKEN"
check_secret "GitHub" "DAISY_SECRET_GITHUB_TOKEN"

echo ""
echo "Note: Only showing first 8 characters of tokens for security"
