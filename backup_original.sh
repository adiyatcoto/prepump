#!/bin/bash
# Backup Original Code Script
# Run this to backup your working version

BACKUP_DIR="backup_$(date +%Y%m%d_%H%M%S)"

echo "Creating backup in: $BACKUP_DIR"
mkdir -p "$BACKUP_DIR"

# Backup all source files
cp -r cmd "$BACKUP_DIR/"
cp -r internal "$BACKUP_DIR/"
cp go.mod "$BACKUP_DIR/"
cp go.sum "$BACKUP_DIR/"

echo "✓ Backup created in: $BACKUP_DIR"
echo ""
echo "To RESTORE from backup:"
echo "  rm -rf cmd internal go.mod go.sum"
echo "  cp -r $BACKUP_DIR/* ."
echo ""
echo "Backup location: $(pwd)/$BACKUP_DIR"
