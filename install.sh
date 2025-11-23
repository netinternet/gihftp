#!/bin/bash

# LOG-FTP-MERGER Installation Script
# This script installs the gihftp binary and configuration files

set -e

BINARY="gihftp"
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc"
CONFIG_FILE="gihftp.conf"
SERVICE_FILE="gihftp.service"
SYSTEMD_DIR="/etc/systemd/system"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=================================================="
echo "   LOG-FTP-MERGER Installation Script"
echo "=================================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}❌ This script must be run as root (use sudo)${NC}"
    exit 1
fi

# Check if binary exists
if [ ! -f "${BINARY}" ]; then
    echo -e "${RED}❌ Binary '${BINARY}' not found in current directory${NC}"
    echo "   Please run this script from the extracted release directory"
    exit 1
fi

echo -e "${GREEN}✓${NC} Binary found: ${BINARY}"
echo ""

# 1. Install binary
echo "📦 Installing binary..."
cp "${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
echo -e "${GREEN}✓${NC} Binary installed to: ${INSTALL_DIR}/${BINARY}"
echo ""

# 2. Install or update config
if [ -f "${CONFIG_DIR}/${CONFIG_FILE}" ]; then
    echo -e "${YELLOW}⚠${NC}  Config file already exists: ${CONFIG_DIR}/${CONFIG_FILE}"
    read -p "   Do you want to backup and replace it? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        BACKUP_FILE="${CONFIG_DIR}/${CONFIG_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
        cp "${CONFIG_DIR}/${CONFIG_FILE}" "${BACKUP_FILE}"
        echo -e "${GREEN}✓${NC} Backup created: ${BACKUP_FILE}"

        if [ -f "gihftp.conf.example" ]; then
            cp gihftp.conf.example "${CONFIG_DIR}/${CONFIG_FILE}"
            echo -e "${GREEN}✓${NC} Config file updated: ${CONFIG_DIR}/${CONFIG_FILE}"
        fi
    else
        echo -e "${YELLOW}⚠${NC}  Keeping existing config file"
    fi
else
    if [ -f "gihftp.conf.example" ]; then
        cp gihftp.conf.example "${CONFIG_DIR}/${CONFIG_FILE}"
        echo -e "${GREEN}✓${NC} Config file installed: ${CONFIG_DIR}/${CONFIG_FILE}"
        echo -e "${YELLOW}⚠${NC}  Please edit the config file: sudo nano ${CONFIG_DIR}/${CONFIG_FILE}"
    else
        echo -e "${YELLOW}⚠${NC}  Config example not found, skipping config installation"
    fi
fi
echo ""

# 3. Create working directory (optional)
WORK_DIR="/var/lib/gihftp"
if [ ! -d "${WORK_DIR}" ]; then
    mkdir -p "${WORK_DIR}"
    chmod 755 "${WORK_DIR}"
    echo -e "${GREEN}✓${NC} Working directory created: ${WORK_DIR}"
else
    echo -e "${GREEN}✓${NC} Working directory exists: ${WORK_DIR}"
fi
echo ""

# 4. Create log directory
LOG_DIR="/var/log/gihftp"
if [ ! -d "${LOG_DIR}" ]; then
    mkdir -p "${LOG_DIR}"
    chmod 755 "${LOG_DIR}"
    echo -e "${GREEN}✓${NC} Log directory created: ${LOG_DIR}"
else
    echo -e "${GREEN}✓${NC} Log directory exists: ${LOG_DIR}"
fi
echo ""

# 5. Install systemd service (optional)
read -p "📋 Do you want to install systemd service? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    cat > "${SYSTEMD_DIR}/${SERVICE_FILE}" << 'EOF'
[Unit]
Description=LOG FTP MERGER Log Upload Service
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/bin/gihftp --config=/etc/gihftp.conf
Environment="FTP_PASSWORD="
StandardOutput=append:/var/log/gihftp/gihftp.log
StandardError=append:/var/log/gihftp/gihftp.log
User=root

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    echo -e "${GREEN}✓${NC} Systemd service installed: ${SERVICE_FILE}"
    echo -e "${YELLOW}⚠${NC}  Edit service file to set FTP_PASSWORD: sudo nano ${SYSTEMD_DIR}/${SERVICE_FILE}"
    echo ""
    echo "   To enable service:"
    echo "     sudo systemctl enable ${SERVICE_FILE}"
    echo ""
    echo "   To start service:"
    echo "     sudo systemctl start ${SERVICE_FILE}"
    echo ""
    echo "   To check status:"
    echo "     sudo systemctl status ${SERVICE_FILE}"
    echo ""
fi

# 6. Setup cron job (optional)
read -p "⏰ Do you want to setup weekly cron job? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    CRON_LINE="0 3 * * 1 /usr/bin/gihftp --config=/etc/gihftp.conf >> /var/log/gihftp/gihftp.log 2>&1"

    # Check if cron job already exists
    if crontab -l 2>/dev/null | grep -q "gihftp"; then
        echo -e "${YELLOW}⚠${NC}  Cron job already exists"
    else
        (crontab -l 2>/dev/null; echo "${CRON_LINE}") | crontab -
        echo -e "${GREEN}✓${NC} Cron job added (runs every Monday at 3 AM)"
        echo "   View cron jobs: crontab -l"
        echo "   Edit cron jobs: crontab -e"
    fi
    echo ""
fi

# 7. Test installation
echo "🧪 Testing installation..."
if command -v gihftp &> /dev/null; then
    VERSION_OUTPUT=$(gihftp --help 2>&1 | head -1 || echo "")
    echo -e "${GREEN}✓${NC} gihftp command is available"
    echo ""
    gihftp --help 2>&1 | head -20
else
    echo -e "${RED}❌ gihftp command not found${NC}"
    exit 1
fi
echo ""

# 8. Summary
echo "=================================================="
echo "   ✨ Installation Complete!"
echo "=================================================="
echo ""
echo "📂 Installed files:"
echo "   Binary:      ${INSTALL_DIR}/${BINARY}"
echo "   Config:      ${CONFIG_DIR}/${CONFIG_FILE}"
echo "   Work Dir:    ${WORK_DIR}"
echo "   Logs:        ${LOG_DIR}"
echo ""
echo "🚀 Next steps:"
echo "   1. Edit config: sudo nano ${CONFIG_DIR}/${CONFIG_FILE}"
echo "   2. Set password: export FTP_PASSWORD='your_password'"
echo "   3. Test run: sudo gihftp --config=${CONFIG_DIR}/${CONFIG_FILE} --log-level=debug"
echo ""
echo "📖 Documentation:"
echo "   View help: gihftp --help"
echo "   Read README: cat README.md"
echo ""
echo "🗑️  To uninstall: sudo ./uninstall.sh"
echo ""
echo "=================================================="
