#!/bin/bash

# LogWrap Test Commands Examples
# Run these commands to test different aspects of logwrap

echo "=== LogWrap Test Commands ==="
echo ""

echo "1. Basic test with echo:"
echo "   logwrap echo 'Hello World'"
echo ""

echo "2. Test with stdout and stderr:"
echo "   logwrap sh -c \"echo 'This is stdout'; echo 'This is stderr' >&2\""
echo ""

echo "3. Test with configuration file:"
echo "   logwrap -config examples/basic.yaml echo 'Configured output'"
echo ""

echo "4. Test with custom template:"
echo "   logwrap -template '[{timestamp}] ' echo 'Custom template'"
echo ""

echo "5. Test with long-running command:"
echo "   logwrap ping -c 3 google.com"
echo ""

echo "6. Test with command that produces mixed output:"
echo "   logwrap ls -la /nonexistent 2>/dev/null || logwrap ls -la /"
echo ""

echo "7. Test with make command (if Makefile exists):"
echo "   logwrap make help"
echo ""

echo "8. Test with no colors:"
echo "   logwrap -colors=false echo 'No colors'"
echo ""

echo "9. Test with UTC timestamps:"
echo "   logwrap -utc echo 'UTC timestamp'"
echo ""

echo "10. Test with minimal configuration:"
echo "    logwrap -config examples/minimal.yaml echo 'Minimal setup'"
echo ""

echo "11. Test with advanced configuration:"
echo "    logwrap -config examples/advanced.yaml echo 'Advanced setup'"
echo ""

echo "12. Test error detection:"
echo "    logwrap sh -c \"echo 'INFO: Starting process'; echo 'ERROR: Something failed' >&2; echo 'WARN: This is a warning'\""
echo ""

echo "13. Test long output:"
echo "    logwrap find /usr -name '*.so' -type f 2>/dev/null | head -20"
echo ""

echo "14. Test with -- separator:"
echo "    logwrap -- sh -c 'echo \"Command with -- separator\"'"
echo ""

echo "Run any of these commands to test logwrap functionality!"