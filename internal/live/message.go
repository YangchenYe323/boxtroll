package live

import (
	"encoding/binary"
	"io"
)

type MessageType uint16

const (
	MessageTypeUncompressedNormal    MessageType = 0
	MessageTypeUncompressedOperation MessageType = 1
	MessageTypeZlibNormal            MessageType = 2
	MessageTypeBrotliNormal          MessageType = 3
)

type Op uint32

const (
	OpHeartbeat      Op = 2
	OpHeartbeatReply Op = 3
	OpNormal         Op = 5
	OpAuth           Op = 7
	OpAuthReply      Op = 8
)

// Header of the bilibili live danmu message:
// +--------------------------------------------------------------------------+
// | Total Length | Header Length | Type    | Op Code | Sequence ID | Message |
// +--------------------------------------------------------------------------+
// | 4 bytes      | 2 bytes       | 2 bytes | 4 bytes | 4 bytes     | N bytes |
// +--------------------------------------------------------------------------+
type MessageHeader struct {
	TotalLength  uint32      // Message length + Header length
	HeaderLength uint16      // Header length (always 16)
	Type         MessageType // Message type
	OpCode       Op          // Operation code
	SequenceID   uint32      // Sequence ID
}

func (m *MessageHeader) Write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, m.TotalLength); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.HeaderLength); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.OpCode); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, m.SequenceID); err != nil {
		return err
	}
	return nil
}

func (m *MessageHeader) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &m.TotalLength); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &m.HeaderLength); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &m.Type); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &m.OpCode); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &m.SequenceID); err != nil {
		return err
	}
	return nil
}

// A faked enum representing all possible messages
type Message struct {
	Cmd      string           `json:"cmd"`
	SendGift *SendGiftMessage `json:"send_gift,omitempty"`
}

type AuthMessage struct {
	Uid      int64  `json:"uid"`
	RoomID   int64  `json:"roomid"`
	ProtoVer int64  `json:"protover"` // Always 3
	Platform string `json:"platform"` // Always "web"
	Type     int64  `json:"type"`     // Always 2
	Key      string `json:"key"`      // The token of the user
}

type SendGiftMessage struct {
	GiftID    int64      `json:"giftId"`
	GiftName  string     `json:"giftName"`
	Num       int64      `json:"num"`
	Price     int64      `json:"price"`
	BlindGift *BlindGift `json:"blind_gift,omitempty"`
	UID       int64      `json:"uid"`
	UName     string     `json:"uname"`
}

type BlindGift struct {
	GiftTipPrice      int64  `json:"gift_tip_price"`
	OriginalGiftID    int64  `json:"original_gift_id"`
	OriginalGiftName  string `json:"original_gift_name"`
	OriginalGiftPrice int64  `json:"original_gift_price"`
}
