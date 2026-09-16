package cli

import (
	"fmt"
)

const Version = "1.0.0"

// PrintBanner outputs the Fling startup banner.
func PrintBanner() {
	banner := `
  ______ _ _             
 |  ____| (_)            
 | |__  | |_ _ __   __ _ 
 |  __| | | | '_ \ / _` + "`" + ` |
 | |    | | | | | | (_| |
 |_|    |_|_|_| |_|\__, |
                    __/ |
                   |___/ 
`
	fmt.Println(banner)
	fmt.Printf(" Fling CLI - P2P File Transfer & Messaging v%s\n", Version)
	fmt.Println(" ==================================================")
}

// PrintHelp outputs the CLI usage guide.
func PrintHelp() {
	help := `Fling - Fast, Reliable Peer-to-Peer File Transfer & Messaging

USAGE:
  fling                                  Start interactive node & peer discovery
  fling send <file> --to <peer>          Send a file directly to a peer
  fling msg "<text>" --to <peer>         Send a text message directly to a peer
  fling peers                            Discover and display active peers
  fling version                          Show version information
  fling uninstall                        Remove Fling from this machine

FLAGS:
  --to, -t <peer>                        Target peer (Hostname, IP:Port, or Session ID)
  --port, -p <port>                      UDP port to listen on (default: 9999)
  --help, -h                             Show this help message
  --version, -v                          Show version information

EXAMPLES:
  # Start listening node on default port 9999
  fling

  # Start listening node on custom port
  fling --port 9998

  # Send a message to a peer
  fling msg "Hello there!" --to 127.0.0.1:9999
  fling msg "Hello there!" --to alice-laptop

  # Send a file to a peer
  fling send document.pdf --to 127.0.0.1:9999
  fling send photos.zip --to bob-desktop

  # Uninstall Fling
  fling uninstall
`
	fmt.Println(help)
}
