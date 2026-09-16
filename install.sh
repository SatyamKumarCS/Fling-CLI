#!/usr/bin/env bash
# ==============================================================================
# Fling CLI - Installer Script
# Fast, Reliable Peer-to-Peer File Transfer & Messaging
# ==============================================================================

set -e

# Color definitions (ANSI 256 / High Contrast)
COLOR_RESET="\033[0m"
COLOR_BOLD="\033[1m"
COLOR_DIM="\033[2m"
COLOR_PURPLE="\033[38;5;141m"
COLOR_CYAN="\033[38;5;51m"
COLOR_GREEN="\033[38;5;48m"
COLOR_YELLOW="\033[38;5;221m"
COLOR_RED="\033[38;5;196m"

print_banner() {
    echo -e "${COLOR_PURPLE}${COLOR_BOLD}"
    cat << "EOF"
  ███████╗██╗     ██╗███╗   ██╗ ██████╗ 
  ██╔════╝██║     ██║████╗  ██║██╔════╝ 
  █████╗  ██║     ██║██╔██╗ ██║██║  ███╗
  ██╔══╝  ██║     ██║██║╚██╗██║██║   ██║
  ██║     ███████╗██║██║ ╚████║╚██████╔╝
  ╚═╝     ╚══════╝╚═╝╚═╝  ╚═══╝ ╚═════╝ 
EOF
    echo -e "${COLOR_CYAN}  P2P File Transfer & Messaging for your Terminal${COLOR_RESET}"
    echo -e "${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}\n"
}

print_banner

# Step 1: Check prerequisites
echo -e "${COLOR_CYAN}${COLOR_BOLD}[1/4]${COLOR_RESET} ${COLOR_BOLD}Checking system prerequisites...${COLOR_RESET}"
if ! command -v go &> /dev/null; then
    echo -e "  ${COLOR_RED}✗ Go compiler is not installed.${COLOR_RESET}"
    echo -e "    Please install Go (1.20+) from: ${COLOR_BOLD}https://go.dev/dl/${COLOR_RESET}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
GO_OS=$(go env GOOS)
GO_ARCH=$(go env GOARCH)
echo -e "  ${COLOR_GREEN}✓${COLOR_RESET} Found Go: ${COLOR_BOLD}${GO_VERSION}${COLOR_RESET} (${GO_OS}/${GO_ARCH})"

# Step 2: Determine destination path
echo -e "\n${COLOR_CYAN}${COLOR_BOLD}[2/4]${COLOR_RESET} ${COLOR_BOLD}Determining installation destination...${COLOR_RESET}"

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

echo -e "  ${COLOR_GREEN}✓${COLOR_RESET} Target path: ${COLOR_BOLD}${TARGET_PATH}${COLOR_RESET}"

# Step 3: Compile Fling binary
echo -e "\n${COLOR_CYAN}${COLOR_BOLD}[3/4]${COLOR_RESET} ${COLOR_BOLD}Compiling Fling binary...${COLOR_RESET}"

if [ -f "go.mod" ] && [ -d "cmd/fling" ]; then
    echo -e "  ${COLOR_DIM}→ Compiling from local repository...${COLOR_RESET}"
    go build -ldflags="-s -w" -o fling ./cmd/fling
    COMPILED_BIN="./fling"
else
    TMP_DIR=$(mktemp -d)
    echo -e "  ${COLOR_DIM}→ Fetching latest release from GitHub...${COLOR_RESET}"
    git clone --quiet --depth 1 https://github.com/SatyamKumarCS/Fling-CLI.git "$TMP_DIR"
    (cd "$TMP_DIR" && go build -ldflags="-s -w" -o fling ./cmd/fling)
    COMPILED_BIN="${TMP_DIR}/fling"
fi

if [ ! -f "$COMPILED_BIN" ]; then
    echo -e "  ${COLOR_RED}✗ Build failed: Binary was not generated.${COLOR_RESET}"
    exit 1
fi
echo -e "  ${COLOR_GREEN}✓${COLOR_RESET} Build succeeded (${COLOR_BOLD}fling v1.0.0${COLOR_RESET})"

# Step 4: Install binary with atomic permissions
echo -e "\n${COLOR_CYAN}${COLOR_BOLD}[4/4]${COLOR_RESET} ${COLOR_BOLD}Installing binary to ${TARGET_PATH}...${COLOR_RESET}"

if [ "$USE_SUDO" = true ]; then
    echo -e "  ${COLOR_YELLOW}Elevated permissions required for /usr/local/bin:${COLOR_RESET}"
    sudo install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
else
    install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
fi

# Clean up temp directory
if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
fi

echo -e "  ${COLOR_GREEN}✓${COLOR_RESET} Installed executable with permissions 0755"

# Verification & Summary Banner
echo -e "\n${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}"
echo -e "  ${COLOR_GREEN}${COLOR_BOLD}✓ Fling successfully installed and ready to use!${COLOR_RESET}"
echo -e "${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}\n"

echo -e "  ${COLOR_BOLD}🚀 QUICK START:${COLOR_RESET}"
echo -e "    ${COLOR_CYAN}fling${COLOR_RESET}                           Launch interactive TUI dashboard"
echo -e "    ${COLOR_CYAN}fling send <file> --to <peer>${COLOR_RESET}   Send file directly to a peer"
echo -e "    ${COLOR_CYAN}fling msg \"<text>\" --to <peer>${COLOR_RESET}  Send direct instant message"
echo -e "    ${COLOR_CYAN}fling peers${COLOR_RESET}                     Scan and list active LAN peers\n"

echo -e "  ${COLOR_BOLD}🔍 VERIFY INSTALLATION:${COLOR_RESET}"
echo -e "    ${COLOR_CYAN}fling version${COLOR_RESET}                   Check installed version"
echo -e "    ${COLOR_CYAN}which fling${COLOR_RESET}                     Show binary location in \$PATH"
echo -e "    ${COLOR_CYAN}fling --help${COLOR_RESET}                    Display command manual & flags\n"

echo -e "  ${COLOR_BOLD}🗑️  UNINSTALLATION:${COLOR_RESET}"
echo -e "    ${COLOR_CYAN}fling uninstall${COLOR_RESET}                 Remove Fling from this system"
echo -e "    ${COLOR_CYAN}make uninstall${COLOR_RESET}                  Remove Fling via Makefile\n"

if ! command -v fling &> /dev/null; then
    echo -e "  ${COLOR_YELLOW}[NOTE] ${INSTALL_DIR} is not in your current PATH.${COLOR_RESET}"
    echo -e "  Add it to your shell profile (~/.zshrc or ~/.bashrc):"
    echo -e "    ${COLOR_BOLD}export PATH=\"${INSTALL_DIR}:\$PATH\"${COLOR_RESET}"
    echo -e "  Then run: ${COLOR_BOLD}source ~/.zshrc${COLOR_RESET}\n"
fi

echo -e "${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}"
