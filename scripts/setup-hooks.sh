#!/bin/bash

# Setup git hooks for Raioz project

set -e

HOOKS_DIR=".git/hooks"
PRE_COMMIT_HOOK="$HOOKS_DIR/pre-commit"

if [ ! -d "$HOOKS_DIR" ]; then
    echo "❌ .git/hooks directory not found. Are you in a git repository?"
    exit 1
fi

echo "📝 Setting up pre-commit hook..."

cat > "$PRE_COMMIT_HOOK" << 'EOF'
#!/bin/bash

# Pre-commit hook for Raioz
# Checks code standards before allowing commit

set -e

echo "🔍 Running pre-commit checks..."

# Check if scripts directory exists
if [ -f "scripts/check-code-standards.sh" ]; then
    ./scripts/check-code-standards.sh
else
    echo "⚠️  Skipping code standards check (scripts not found)"
fi

# Run gofmt check
echo ""
echo "📝 Checking code formatting..."
UNFORMATTED=$(gofmt -l . | grep -v vendor || true)
if [ -z "$UNFORMATTED" ]; then
    echo "✅ Code is properly formatted"
else
    echo "❌ Code is not formatted. Run: make format"
    echo "Unformatted files:"
    echo "$UNFORMATTED"
    exit 1
fi

# Run basic tests (quick check)
echo ""
echo "🧪 Running quick tests..."
if go test ./... -short > /dev/null 2>&1; then
    echo "✅ All tests pass"
else
    echo "❌ Some tests failed. Fix before committing."
    exit 1
fi

echo ""
echo "✅ Pre-commit checks passed!"
EOF

chmod +x "$PRE_COMMIT_HOOK"

echo "✅ Pre-commit hook installed!"
echo ""
echo "To disable the hook temporarily, run:"
echo "  git commit --no-verify"
