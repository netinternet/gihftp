#!/bin/bash

# IMPORTANT: This is a development deployment script
# For production, use: ./make-release.sh and ./install.sh

# Configure your server details here
SERVER="${DEPLOY_SERVER:-root@your-server.example.com}"
BINARY="gihftp"
REMOTE_BIN="/usr/bin"
REMOTE_CONFIG="/etc/gihftp.conf"

echo "🔥 Building binary for Linux..."
GOOS=linux GOARCH=amd64 go build -o $BINARY

if [ $? -ne 0 ]; then
    echo "❌ Build failed. Exiting."
    exit 1
fi

echo "🚀 Copying binary to $SERVER ..."
scp $BINARY $SERVER:$REMOTE_BIN/

echo "🚀 Setting executable permission..."
ssh $SERVER "chmod +x $REMOTE_BIN/$BINARY"

echo "📁 Checking /etc/gihftp.conf ..."
if ssh $SERVER "[ -f $REMOTE_CONFIG ]"; then
    echo "✔ Config already exists. Skipping copy."
else
    echo "📁 Copying config file..."
    scp gihftp.conf $SERVER:$REMOTE_CONFIG
    echo "✔ Config copied."
fi

echo "✨ Deployment complete!"