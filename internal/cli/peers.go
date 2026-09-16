package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
)

// PrintPeers displays a list of discovered peers in a clean tabular format.
func PrintPeers(peers []discovery.Peer) {
	fmt.Println()
	if len(peers) == 0 {
		fmt.Println("--- Active Discovered Peers (0) ---")
		fmt.Println("No active peers discovered yet on LAN / Localhost.")
		fmt.Println("  -> To test locally: open a second terminal and run: ./fling --port 9998")
		fmt.Println("  -> To share with a friend: have them run './fling' on the same Wi-Fi")
		fmt.Println("--------------------------------------------------")
		fmt.Println()
		return
	}

	fmt.Printf("--- Active Discovered Peers (%d) ---\n", len(peers))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PEER #\tHOSTNAME\tIP ADDRESS\tSESSION ID\tLAST SEEN")
	fmt.Fprintln(w, "------\t--------\t----------\t----------\t---------")

	now := time.Now()
	for i, peer := range peers {
		ago := now.Sub(peer.LastSeen).Truncate(time.Second)
		fmt.Fprintf(
			w,
			"%d\t%s\t%s\t%s\t%s ago\n",
			i+1,
			peer.Hostname,
			peer.Addr(),
			peer.SessionID,
			ago,
		)
	}
	w.Flush()
	fmt.Println("--------------------------------------------------")
	fmt.Println()
}
