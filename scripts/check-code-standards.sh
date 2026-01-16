#!/bin/bash

# Script to check code standards compliance
# Checks: line count, line length, and runs linter

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

MAX_LINES=400
MAX_LINE_LENGTH=120
ERRORS=0

echo "🔍 Checking code standards..."

# Check file line count
echo ""
echo "📏 Checking file line count (max: $MAX_LINES lines)..."
LONG_FILES=$(find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/.ia/*" ! -path "*/scripts/*" -exec sh -c 'lines=$(wc -l < "$1"); if [ "$lines" -gt '"$MAX_LINES"' ]; then echo "$1:$lines"; fi' _ {} \;)

if [ -n "$LONG_FILES" ]; then
    echo -e "${RED}❌ Files exceeding $MAX_LINES lines found:${NC}"
    echo "$LONG_FILES" | while IFS=: read -r file lines; do
        echo -e "  ${RED}$file${NC}: ${YELLOW}$lines lines${NC}"
    done
    ERRORS=$((ERRORS + 1))
else
    echo -e "${GREEN}✅ All files are under $MAX_LINES lines${NC}"
fi

# Check line length
echo ""
echo "📐 Checking line length (max: $MAX_LINE_LENGTH characters)..."
LONG_LINES=$(find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/.ia/*" ! -path "*/scripts/*" -exec awk -v max="$MAX_LINE_LENGTH" 'length > max {print FILENAME":"NR":"length($0)}' {} \;)

if [ -n "$LONG_LINES" ]; then
    echo -e "${RED}❌ Lines exceeding $MAX_LINE_LENGTH characters found:${NC}"
    echo "$LONG_LINES" | head -20 | while IFS=: read -r file line chars; do
        echo -e "  ${RED}$file:$line${NC}: ${YELLOW}$chars chars${NC}"
    done
    if [ "$(echo "$LONG_LINES" | wc -l)" -gt 20 ]; then
        echo -e "  ${YELLOW}... and more (showing first 20)${NC}"
    fi
    ERRORS=$((ERRORS + 1))
else
    echo -e "${GREEN}✅ All lines are under $MAX_LINE_LENGTH characters${NC}"
fi

# Check for files with multiple responsibilities (heuristic: multiple exported types/functions)
echo ""
echo "🎯 Checking file focus (files should have single responsibility)..."
MULTI_PURPOSE=$(find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/.ia/*" ! -path "*/scripts/*" -exec sh -c '
    file="$1"
    exported_types=$(grep -c "^type [A-Z]" "$file" 2>/dev/null || true)
    exported_funcs=$(grep -c "^func [A-Z]" "$file" 2>/dev/null || true)
    if [ -z "$exported_types" ]; then exported_types=0; fi
    if [ -z "$exported_funcs" ]; then exported_funcs=0; fi
    total=$((exported_types + exported_funcs))
    if [ "$total" -gt 10 ]; then
        echo "$file:$total"
    fi
' _ {} \;)

if [ -n "$MULTI_PURPOSE" ]; then
    echo -e "${YELLOW}⚠️  Files with many exported symbols (might need splitting):${NC}"
    echo "$MULTI_PURPOSE" | while IFS=: read -r file count; do
        echo -e "  ${YELLOW}$file${NC}: ${YELLOW}$count exported symbols${NC}"
    done
    echo -e "  ${YELLOW}Note: This is a heuristic, review manually${NC}"
fi

# Summary
echo ""
if [ "$ERRORS" -eq 0 ]; then
    echo -e "${GREEN}✅ All code standards checks passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Found $ERRORS violation(s)${NC}"
    exit 1
fi
