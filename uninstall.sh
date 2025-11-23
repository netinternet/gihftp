#!/bin/bash

# LOG-FTP-MERGER Uninstallation Script
# This script removes all gihftp files and configurations

set -e

BINARY="gihftp"
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc"
CONFIG_FILE="gihftp.conf"
SERVICE_FILE="gihftp.service"
SYSTEMD_DIR="/etc/systemd/system"
WORK_DIR="/var/lib/gihftp"
LOG_DIR="/var/log/gihftp"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=================================================="
echo "   LOG-FTP-MERGER Uninstallation Script"
echo "=================================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}❌ This script must be run as root (use sudo)${NC}"
    exit 1
fi

# Confirm uninstallation
echo -e "${YELLOW}⚠️  WARNING: This will remove gihftp and all related files${NC}"
echo ""
read -p "Are you sure you want to uninstall? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstallation cancelled"
    exit 0
fi
echo ""

# 1. Stop and disable systemd service
if [ -f "${SYSTEMD_DIR}/${SERVICE_FILE}" ]; then
    echo "🛑 Stopping systemd service..."
    systemctl stop "${SERVICE_FILE}" 2>/dev/null || true
    systemctl disable "${SERVICE_FILE}" 2>/dev/null || true
    rm -f "${SYSTEMD_DIR}/${SERVICE_FILE}"
    systemctl daemon-reload
    echo -e "${GREEN}✓${NC} Systemd service removed"
else
    echo -e "${YELLOW}⚠${NC}  No systemd service found"
fi
echo ""

# 2. Remove cron jobs
echo "⏰ Removing cron jobs..."
if crontab -l 2>/dev/null | grep -q "gihftp"; then
    crontab -l 2>/dev/null | grep -v "gihftp" | crontab - || true
    echo -e "${GREEN}✓${NC} Cron jobs removed"
else
    echo -e "${YELLOW}⚠${NC}  No cron jobs found"
fi
echo ""

# 3. Remove binary
if [ -f "${INSTALL_DIR}/${BINARY}" ]; then
    rm -f "${INSTALL_DIR}/${BINARY}"
    echo -e "${GREEN}✓${NC} Binary removed: ${INSTALL_DIR}/${BINARY}"
else
    echo -e "${YELLOW}⚠${NC}  Binary not found: ${INSTALL_DIR}/${BINARY}"
fi
echo ""

# 4. Handle config file
if [ -f "${CONFIG_DIR}/${CONFIG_FILE}" ]; then
    read -p "📝 Do you want to remove config file? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Backup before removing
        BACKUP_FILE="${CONFIG_DIR}/${CONFIG_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
        cp "${CONFIG_DIR}/${CONFIG_FILE}" "${BACKUP_FILE}"
        echo -e "${GREEN}✓${NC} Config backed up: ${BACKUP_FILE}"

        rm -f "${CONFIG_DIR}/${CONFIG_FILE}"
        echo -e "${GREEN}✓${NC} Config removed: ${CONFIG_DIR}/${CONFIG_FILE}"
    else
        echo -e "${YELLOW}⚠${NC}  Keeping config file: ${CONFIG_DIR}/${CONFIG_FILE}"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Config file not found"
fi
echo ""

# 5. Handle working directory
if [ -d "${WORK_DIR}" ]; then
    read -p "📂 Do you want to remove working directory? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "${WORK_DIR}"
        echo -e "${GREEN}✓${NC} Working directory removed: ${WORK_DIR}"
    else
        echo -e "${YELLOW}⚠${NC}  Keeping working directory: ${WORK_DIR}"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Working directory not found"
fi
echo ""

# 6. Handle log directory
if [ -d "${LOG_DIR}" ]; then
    read -p "📋 Do you want to remove log directory? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Backup logs before removing
        if [ "$(ls -A ${LOG_DIR})" ]; then
            BACKUP_LOG="${LOG_DIR}_backup_$(date +%Y%m%d_%H%M%S).tar.gz"
            tar -czf "${BACKUP_LOG}" -C "${LOG_DIR}" .
            echo -e "${GREEN}✓${NC} Logs backed up: ${BACKUP_LOG}"
        fi

        rm -rf "${LOG_DIR}"
        echo -e "${GREEN}✓${NC} Log directory removed: ${LOG_DIR}"
    else
        echo -e "${YELLOW}⚠${NC}  Keeping log directory: ${LOG_DIR}"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Log directory not found"
fi
echo ""

# 7. Remove config backups (optional)
CONFIG_BACKUPS=$(find "${CONFIG_DIR}" -name "${CONFIG_FILE}.backup.*" 2>/dev/null || true)
if [ -n "${CONFIG_BACKUPS}" ]; then
    echo "🗂️  Found config backups:"
    echo "${CONFIG_BACKUPS}"
    echo ""
    read -p "Do you want to remove backup config files? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        find "${CONFIG_DIR}" -name "${CONFIG_FILE}.backup.*" -delete
        echo -e "${GREEN}✓${NC} Backup config files removed"
    else
        echo -e "${YELLOW}⚠${NC}  Keeping backup config files"
    fi
    echo ""
fi

# 8. Verify uninstallation
echo "🧪 Verifying uninstallation..."
if command -v gihftp &> /dev/null; then
    echo -e "${RED}❌ gihftp command still available (might be in different location)${NC}"
    which gihftp
else
    echo -e "${GREEN}✓${NC} gihftp command removed"
fi
echo ""

# 9. Summary
echo "=================================================="
echo "   ✨ Uninstallation Complete!"
echo "=================================================="
echo ""
echo "📋 Removed items:"
echo "   ✓ Binary (if existed)"
echo "   ✓ Systemd service (if existed)"
echo "   ✓ Cron jobs (if existed)"
echo ""

if [ -f "${CONFIG_DIR}/${CONFIG_FILE}" ]; then
    echo "📂 Remaining files:"
    echo "   - Config: ${CONFIG_DIR}/${CONFIG_FILE}"
fi

if [ -d "${WORK_DIR}" ]; then
    echo "   - Work dir: ${WORK_DIR}"
fi

if [ -d "${LOG_DIR}" ]; then
    echo "   - Logs: ${LOG_DIR}"
fi

# Check for any backups
REMAINING_BACKUPS=$(find "${CONFIG_DIR}" -name "${CONFIG_FILE}.backup.*" 2>/dev/null || true)
if [ -n "${REMAINING_BACKUPS}" ]; then
    echo "   - Backups: ${CONFIG_DIR}/${CONFIG_FILE}.backup.*"
fi

LOG_BACKUPS=$(find / -maxdepth 2 -name "${LOG_DIR##*/}_backup_*.tar.gz" 2>/dev/null || true)
if [ -n "${LOG_BACKUPS}" ]; then
    echo "   - Log backups: ${LOG_BACKUPS}"
fi

echo ""
echo "🔄 To reinstall: sudo ./install.sh"
echo ""
echo "=================================================="
