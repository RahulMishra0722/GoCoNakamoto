package MessageType

type MessageType int

const (
	RequestLatestBlock   MessageType = 0x01
	SendLatestBlock      MessageType = 0x02
	BroadcastTransaction MessageType = 0x03
	PeerDiscovery        MessageType = 0x04
	Version              MessageType = 0x04
)
