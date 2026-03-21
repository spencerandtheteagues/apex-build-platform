#!/bin/sh
set -eu

DIST_DIR="dist"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

cp index.html "$DIST_DIR"/
cp styles.css "$DIST_DIR"/
cp presentation.js "$DIST_DIR"/
cp apex-build-logo-transparent.png "$DIST_DIR"/
cp -R screens "$DIST_DIR"/screens
