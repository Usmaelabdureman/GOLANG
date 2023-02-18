package torrent

import "bittorrent/tracker"

// Which source informed us about that peer
type PeerSource byte

const (
	//The user manually added this peer
	SourceUser PeerSource = iota
	//It was an incoming connection
	SourceIncoming
	//The peer was given to us by DHT
	SourceDHT
	//The peer was given to us by a tracker
	SourceTracker
)

// Holds basic information about a peer
type Peer struct {
	P      tracker.Peer
	Source PeerSource
}
