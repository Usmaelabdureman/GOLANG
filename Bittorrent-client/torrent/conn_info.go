package torrent

import (
	"bittorrent/peer_wire"
	"fmt"
	"time"

	"github.com/anacrolix/missinggo/bitmap"
)

// connInfo sends msgs to conn .It also holds
// some informations like state,bitmap which also conn holds too -
// we dont share, we communicate so we have some duplicate data-.
type connInfo struct {
	t        *Torrent
	peer     Peer
	reserved peer_wire.Reserved
	//we communicate with conn with these channels - conn also has them
	sendC    chan interface{}
	recvC    chan interface{}
	droppedC chan struct{}
	//peer's bitmap
	peerBf  bitmap.Bitmap //also conn has this
	numWant int           //how many pieces are we interested to download from peer
	state   connState     //also conn has this
	stats   connStats
}

func (cn *connInfo) sendMsgToConn(msg interface{}) {
	select {
	case cn.sendC <- msg:
		cn.t.msgsSentToConn++
	case <-cn.droppedC:
		cn.t.droppedConn(cn)
	}
}

func (cn *connInfo) choke() {
	if !cn.state.amChoking {
		cn.sendMsgToConn(&peer_wire.Msg{
			Kind: peer_wire.Choke,
		})
		cn.state.amChoking = !cn.state.amChoking
		if cn.state.isInterested {
			cn.stoppedUploading()
		}
	}
}

func (cn *connInfo) unchoke() {
	if cn.state.amChoking && cn.t.uploadEnabled {
		cn.sendMsgToConn(&peer_wire.Msg{
			Kind: peer_wire.Unchoke,
		})
		cn.state.amChoking = !cn.state.amChoking
		cn.startedUploading()
	}
}

func (cn *connInfo) interested() {
	if cn.numWant <= 0 {
		return
	}
	if !cn.state.amInterested && cn.t.downloadEnabled {
		cn.sendMsgToConn(&peer_wire.Msg{
			Kind: peer_wire.Interested,
		})
		cn.state.amInterested = !cn.state.amInterested
		cn.startedDownloading()
	}
}

func (cn *connInfo) notInterested() {
	if cn.state.amInterested {
		cn.sendMsgToConn(&peer_wire.Msg{
			Kind: peer_wire.NotInterested,
		})
		cn.state.amInterested = !cn.state.amInterested
		if !cn.state.isChoking {
			cn.stoppedDownloading()
		}
	}
}

func (cn *connInfo) have(i int) {
	cn.sendMsgToConn(&peer_wire.Msg{
		Kind:  peer_wire.Have,
		Index: uint32(i),
	})
}

func (cn *connInfo) sendBitfield() {
	cn.sendMsgToConn(cn.t.pieces.ownedPieces.Copy())
}

func (cn *connInfo) sendPort() {
	cn.sendMsgToConn(&peer_wire.Msg{
		Kind: peer_wire.Port,
		Port: cn.t.cl.dhtPort(),
	})
}

func (cn *connInfo) peerInterestChanged() {
	cn.state.isInterested = !cn.state.isInterested
	if cn.state.isInterested {
		if !cn.state.amChoking {
			cn.t.choker.reviewUnchokedPeers()
		}
		cn.startedUploading()
	} else {
		if !cn.state.amChoking {
			cn.t.choker.reviewUnchokedPeers()
			cn.stoppedUploading()
		}
	}
}

func (cn *connInfo) peerChokeChanged() {
	cn.state.isChoking = !cn.state.isChoking
	if cn.state.isChoking {
		if cn.state.amInterested {
			cn.stoppedDownloading()
		}
	} else {
		cn.startedDownloading()
	}
}

// manages if we are interested in peer after a sending us bitfield msg
func (cn *connInfo) reviewInterestsOnBitfield() {
	if !cn.t.haveInfo() || cn.t.haveAll() {
		return
	}
	for i := 0; i < cn.t.numPieces(); i++ {
		if !cn.t.pieces.ownedPieces.Get(i) && cn.peerBf.Get(i) {
			cn.numWant++
		}
	}
	if cn.numWant > 0 {
		cn.interested()
	}
}

// manages if we are interested in peer after sending us a have msg
func (cn *connInfo) reviewInterestsOnHave(i int) {
	if !cn.t.haveInfo() || cn.t.haveAll() {
		return
	}
	if !cn.t.pieces.ownedPieces.Get(i) {
		if cn.numWant <= 0 {
			cn.interested()
		}
		cn.numWant++
	}
}

func (cn *connInfo) durationDownloading() time.Duration {
	if cn.state.canDownload() {
		return cn.stats.sumDownloading + time.Since(cn.stats.lastStartedDownloading)
	}
	return cn.stats.sumDownloading
}

func (cn *connInfo) durationUploading() time.Duration {
	if cn.state.canUpload() {
		return cn.stats.sumUploading + time.Since(cn.stats.lastStartedUploading)
	}
	return cn.stats.sumUploading
}

func (cn *connInfo) startedDownloading() {
	if cn.state.canDownload() {
		cn.stats.lastStartedDownloading = time.Now()
		//Set last piece msg the first time we get into `downloading` state.
		//We didn't got any piece msg but we want to have an initial time to check
		//if we are snubbed.
		if cn.stats.lastReceivedPieceMsg.IsZero() {
			cn.stats.lastReceivedPieceMsg = time.Now()
		}
	}
}

func (cn *connInfo) startedUploading() {
	if cn.state.canUpload() {
		cn.stats.lastStartedUploading = time.Now()
	}
}

func (cn *connInfo) stoppedDownloading() {
	cn.stats.stopDownloading()
}

func (cn *connInfo) stoppedUploading() {
	cn.stats.stopUploading()
}

func (cn *connInfo) isSnubbed() bool {
	if cn.t.haveAll() {
		return false
	}
	prev, curr := cn.stats.snubbed, cn.stats.isSnubbed()
	if curr != prev {
		cn.t.cl.counters.Add("snubbed", 1)
	}
	return curr
}

func (cn *connInfo) peerSeeding() bool {
	if !cn.t.haveInfo() { //we don't know if it has all (maybe he has)
		return false
	}
	return cn.peerBf.Len() == cn.t.numPieces()
}

func (cn *connInfo) rate() float64 {
	safeDiv := func(bytes, dur float64) float64 {
		if dur == 0 {
			return 0
		}
		return bytes / dur
	}
	if cn.t.haveAll() {
		return safeDiv(float64(cn.stats.uploadUsefulBytes), float64(cn.durationUploading()))
	}
	return safeDiv(float64(cn.stats.downloadUsefulBytes), float64(cn.durationDownloading()))
}

func (cn *connInfo) String() string {
	return fmt.Sprintf(`peer seeding: %t
	client interested in %d pieces which peer offers
	downloading for %s
	uploading for %s
	`,
		cn.peerSeeding(),
		cn.numWant, cn.durationDownloading().String(),
		cn.durationUploading().String()) + cn.state.String() + cn.stats.String()
}
