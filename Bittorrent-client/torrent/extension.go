package torrent

import (
	"bittorrent/peer_wire"
)

//TODO:The extensions that we support are
//hardcoded.Change this?

var extensions = peer_wire.Extensions{
	peer_wire.ExtMetadataName: peer_wire.ExtMetadataID,
}

// prepare client's handshake msg for send
func extensionHandshakeMsg(metaSz int64) *peer_wire.Msg {
	return &peer_wire.Msg{
		Kind:       peer_wire.Extended,
		ExtendedID: 0,
		ExtendedMsg: struct {
			ExtMap peer_wire.Extensions `bencode:"m"`
			MetaSz int64                `bencode:"metadata_size" empty:"omit"`
		}{
			ExtMap: extensions,
			MetaSz: metaSz,
		},
	}
}

//func extensionMetadataMsg(){ }
