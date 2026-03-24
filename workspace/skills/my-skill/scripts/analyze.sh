#!/bin/bash
echo "Analyzing: $1"
ls -la "$1" 2>/dev/null || echo "Directory not found"
