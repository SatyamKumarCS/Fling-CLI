#!/usr/bin/env bash
# ==============================================================================
# Fling CLI - Installer Script
# Fast, Reliable Peer-to-Peer File Transfer & Messaging
# ==============================================================================

set -e

# Color definitions
COLOR_RESET="\033[0m"
COLOR_BOLD="\033[1m"
COLOR_PURPLE="\033[38;5;135m"
COLOR_CYAN="\033[38;5;51m"
COLOR_GREEN="\033[38;5;48m"
COLOR_YELLOW="\033[38;5;220m"
COLOR_RED="\033[38;5;196m"

print_banner() {
    echo -e "${COLOR_PURPLE}${COLOR_BOLD}"
    cat << "EOF"
  ______ _ _             
 |  ____| (_)            
 | |__  | |_ _ __   __ _ 
 |  __| | | | '_ \ / _` |
 | |    | | | | | | (_| |
 |_|    |_|_|_| |_|\__, |
                    __/ |
                   |___/ 
EOF
    echo -e "${COLOR_CYAN} Fling CLI - P2P File Transfer & Messaging Installer${COLOR_RESET}"
    echo -e "${COLOR_PURPLE}==================================================${COLOR_RESET}\n"
}

print_banner

# Step 1: Check prerequisites (Go compiler)
echo -e "${COLOR_CYAN}[1/4] Checking prerequisites...${COLOR_RESET}"
if ! command -v go &> /dev/null; then
    echo -e "${COLOR_RED}[ERROR] Go is not installed on this system.${COLOR_RESET}"
    echo -e "Please install Go (1.18+) from: ${COLOR_BOLD}https://go.dev/dl/${COLOR_RESET}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${COLOR_GREEN}✓ Found Go: ${GO_VERSION}${COLOR_RESET}"

if ! command -v git &> /dev/null; then
    echo -e "${COLOR_YELLOW}[!] Git is not installed. Git is recommended for updates.${COLOR_RESET}"
fi

# Step 2: Determine installation directory
echo -e "\n${COLOR_CYAN}[2/4] Determining installation destination...${COLOR_RESET}"

USE_SUDO=false

if [ -d "${HOME}/.local/bin" ] && [[ ":$PATH:" == *":${HOME}/.local/bin:"* ]]; then
    INSTALL_DIR="${HOME}/.local/bin"
    TARGET_PATH="${INSTALL_DIR}/fling"
elif [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
    TARGET_PATH="${INSTALL_DIR}/fling"
elif [ -d "/usr/local/bin" ] && command -v sudo &> /dev/null && [ -t 0 ]; then
    INSTALL_DIR="/usr/local/bin"
    TARGET_PATH="${INSTALL_DIR}/fling"
    USE_SUDO=true
else
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    TARGET_PATH="${INSTALL_DIR}/fling"
fi

echo -e "Target destination: ${COLOR_BOLD}${TARGET_PATH}${COLOR_RESET}"

# Step 3: Build or install Fling binary
echo -e "\n${COLOR_CYAN}[3/4] Compiling Fling binary...${COLOR_RESET}"

# Check if running within the Fling repository
if [ -f "go.mod" ] && [ -d "cmd/fling" ]; then
    echo -e "Building from local source directory..."
    go build -ldflags="-s -w" -o fling ./cmd/fling
    COMPILED_BIN="./fling"
else
    # Remote/Standalone execution - clone to temp directory or go install
    TMP_DIR=$(mktemp -d)
    echo -e "Cloning repository to temporary directory: ${TMP_DIR}..."
    git clone --depth 1 https://github.com/SatyamKumarCS/Fling-CLI.git "$TMP_DIR"
    cd "$TMP_DIR"
    go build -ldflags="-s -w" -o fling ./cmd/fling
    COMPILED_BIN="${TMP_DIR}/fling"
fi

if [ ! -f "$COMPILED_BIN" ]; then
    echo -e "${COLOR_RED}[ERROR] Build failed. Binary was not generated.${COLOR_RESET}"
    exit 1
fi
echo -e "${COLOR_GREEN}✓ Build succeeded.${COLOR_RESET}"

# Step 4: Install binary to PATH
echo -e "\n${COLOR_CYAN}[4/4] Installing binary to ${TARGET_PATH}...${COLOR_RESET}"

if [ "$USE_SUDO" = true ]; then
    echo -e "${COLOR_YELLOW}Elevated permissions required to write to /usr/local/bin:${COLOR_RESET}"
    sudo install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
else
    install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
fi

# Clean up temp dir if created
if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
fi

# Verify installation in PATH
echo -e "\n${COLOR_PURPLE}==================================================${COLOR_RESET}"
if command -v fling &> /dev/null; then
    echo -e "${COLOR_GREEN}${COLOR_BOLD}✓ Fling successfully installed and ready!${COLOR_RESET}\n"
    echo -e "You can now run ${COLOR_CYAN}${COLOR_BOLD}fling${COLOR_RESET} directly from any terminal window:"
    echo -e "  • ${COLOR_BOLD}fling${COLOR_RESET}                           Start full-screen interactive TUI"
    echo -e "  • ${COLOR_BOLD}fling send <file> --to <peer>${COLOR_RESET}   Send file to peer"
    echo -e "  • ${COLOR_BOLD}fling msg \"<text>\" --to <peer>${COLOR_RESET}  Send message to peer"
    echo -e "  • ${COLOR_BOLD}fling peers${COLOR_RESET}                     Scan for active peers"
else
    echo -e "${COLOR_GREEN}✓ Binary copied to: ${TARGET_PATH}${COLOR_RESET}"
    echo -e "${COLOR_YELLOW}[NOTE] ${INSTALL_DIR} is not in your current PATH.${COLOR_RESET}"
    echo -e "Add it to your shell profile (~/.zshrc, ~/.bashrc, or ~/.config/fish/config.fish):"
    echo -e "  ${COLOR_BOLD}export PATH=\"${INSTALL_DIR}:\$PATH\"${COLOR_RESET}"
    echo -e "Then reload your shell: ${COLOR_BOLD}source ~/.zshrc${COLOR_RESET}"
fi
echo -e "${COLOR_PURPLE}==================================================${COLOR_RESET}"
