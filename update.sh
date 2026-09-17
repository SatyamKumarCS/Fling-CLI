#!/usr/bin/env bash
# ==============================================================================
# Fling CLI - Updater Script
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
    echo -e "${COLOR_CYAN}  Fling CLI Auto-Updater${COLOR_RESET}"
    echo -e "${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}\n"
}

print_banner

# Step 1: Detect current version and binary location
CURRENT_VER="unknown"
if command -v fling &> /dev/null; then
    CURRENT_BIN=$(which fling)
    if [ -L "$CURRENT_BIN" ]; then
        RESOLVED=$(readlink "$CURRENT_BIN" || echo "$CURRENT_BIN")
        if [[ "$RESOLVED" != /* ]]; then
            CURRENT_BIN="$(dirname "$CURRENT_BIN")/$RESOLVED"
        else
            CURRENT_BIN="$RESOLVED"
        fi
    fi
    CURRENT_VER=$(fling version 2>/dev/null || echo "v1.0.0")
    echo -e "${COLOR_CYAN}${COLOR_BOLD}[1/3]${COLOR_RESET} Current version: ${COLOR_BOLD}${CURRENT_VER}${COLOR_RESET} (${CURRENT_BIN})"
else
    CURRENT_BIN=""
    echo -e "${COLOR_CYAN}${COLOR_BOLD}[1/3]${COLOR_RESET} Fling is not currently installed. Performing fresh install..."
fi

# Step 2: Fetch and compile latest version
echo -e "\n${COLOR_CYAN}${COLOR_BOLD}[2/3]${COLOR_RESET} ${COLOR_BOLD}Fetching and building latest version...${COLOR_RESET}"

if ! command -v go &> /dev/null; then
    echo -e "  ${COLOR_RED}✗ Go compiler is required to build updates.${COLOR_RESET}"
    echo -e "    Please install Go (1.20+) from: ${COLOR_BOLD}https://go.dev/dl/${COLOR_RESET}"
    exit 1
fi

if [ -f "go.mod" ] && [ -d "cmd/fling" ]; then
    echo -e "  ${COLOR_DIM}→ Compiling from local repository...${COLOR_RESET}"
    go build -ldflags="-s -w" -o fling ./cmd/fling
    COMPILED_BIN="./fling"
else
    TMP_DIR=$(mktemp -d)
    echo -e "  ${COLOR_DIM}→ Cloning latest repository from GitHub (SatyamKumarCS/Fling-CLI)...${COLOR_RESET}"
    git clone --quiet --depth 1 https://github.com/SatyamKumarCS/Fling-CLI.git "$TMP_DIR"
    (cd "$TMP_DIR" && go build -ldflags="-s -w" -o fling ./cmd/fling)
    COMPILED_BIN="${TMP_DIR}/fling"
fi

if [ ! -f "$COMPILED_BIN" ]; then
    echo -e "  ${COLOR_RED}✗ Build failed: Binary was not generated.${COLOR_RESET}"
    exit 1
fi
echo -e "  ${COLOR_GREEN}✓${COLOR_RESET} Build succeeded."

# Step 3: Replace old binary with new version
echo -e "\n${COLOR_CYAN}${COLOR_BOLD}[3/3]${COLOR_RESET} ${COLOR_BOLD}Installing updated binary...${COLOR_RESET}"

if [ -n "$CURRENT_BIN" ] && [ -f "$CURRENT_BIN" ]; then
    TARGET_PATH="$CURRENT_BIN"
elif [ -d "${HOME}/.local/bin" ] && [[ ":$PATH:" == *":${HOME}/.local/bin:"* ]]; then
    TARGET_PATH="${HOME}/.local/bin/fling"
elif [ -w "/usr/local/bin" ]; then
    TARGET_PATH="/usr/local/bin/fling"
else
    TARGET_PATH="${HOME}/.local/bin/fling"
    mkdir -p "${HOME}/.local/bin"
fi

if [ -w "$(dirname "$TARGET_PATH")" ] || [ -w "$TARGET_PATH" ]; then
    install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
else
    echo -e "  ${COLOR_YELLOW}Elevated permissions required for ${TARGET_PATH}:${COLOR_RESET}"
    sudo install -m 755 "$COMPILED_BIN" "$TARGET_PATH"
fi

# Clean up temp directory
if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
fi

NEW_VER=$("$TARGET_PATH" version 2>/dev/null || echo "Fling CLI v1.0.0")

echo -e "\n${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}"
echo -e "  ${COLOR_GREEN}${COLOR_BOLD}✓ Fling successfully updated to ${NEW_VER}!${COLOR_RESET}"
echo -e "  Installed path: ${COLOR_BOLD}${TARGET_PATH}${COLOR_RESET}"
echo -e "${COLOR_PURPLE}  ────────────────────────────────────────────────────────${COLOR_RESET}\n"
