package live

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/rs/zerolog/log"
)

type Stream struct {
	RoomID int64 // Room ID to connect to

	endpoints []*bilibili.LiveEndpoint // A list of endpoints to connect to
	uid       int64                    // User ID of the user
	token     string                   // Auth token for the user
}

func NewStream(
	roomID int64,
	uID int64,
	token string,
	endpoints []*bilibili.LiveEndpoint,
) *Stream {
	s := &Stream{
		RoomID:    roomID,
		uid:       uID,
		token:     token,
		endpoints: endpoints,
	}

	return s
}

func (s *Stream) Run(
	ctx context.Context,
	msgChan chan<- Message, // Send decoded message to the channel
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	nextEndpoint := s.roundRobinEndpointSelector()

	for {
		endpoint := nextEndpoint()

		conn, err := connect(endpoint)
		if err != nil {
			log.Err(err).Msgf("无法连接到弹幕服务器: %s:%d, 5秒后重试其他服务器...", endpoint.Host, endpoint.Port)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Info().Msgf("连接到弹幕服务器: %s:%d", endpoint.Host, endpoint.Port)

		if err := s.driveConnection(ctx, conn, msgChan); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("退出弹幕流")
				conn.Close()
				return
			}

			log.Err(err).Msgf("弹幕服务器连接异常退出, 5秒后重试其他服务器...")
		}

		time.Sleep(5 * time.Second)
	}
}

func (s *Stream) roundRobinEndpointSelector() func() *bilibili.LiveEndpoint {
	curEndpoint := 0
	return func() *bilibili.LiveEndpoint {
		endpoint := s.endpoints[curEndpoint]
		curEndpoint++
		curEndpoint %= len(s.endpoints)
		return endpoint
	}
}

// Drive the life cycle of an established TCP connection until either context is cancelled or the connection is closed.
// Note that the socket is BLOCKING, so if the server blocks forever we will too.
func (s *Stream) driveConnection(ctx context.Context, conn net.Conn, msgChan chan<- Message) error {
	go func() {
		if err := s.authAndHeartbeat(ctx, conn); err != nil {
			log.Err(err).Msg("心跳线程异常退出")
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		messages, err := ReadMessages(conn)
		if err != nil {
			log.Err(err).Msg("读取消息线程异常退出")
			return err
		}

		for _, message := range messages {
			if message == nil {
				continue
			}
			msgChan <- *message
		}

		// Wait for 10ms before reading again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

func (s *Stream) authAndHeartbeat(ctx context.Context, conn net.Conn) error {
	sequenceID := uint32(0)
	// Send auth message
	authMessage := AuthMessage{
		Uid:      s.uid,
		RoomID:   s.RoomID,
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      s.token,
	}

	authMessageBytes, err := json.Marshal(authMessage)
	if err != nil {
		return err
	}

	authMessageHeader := MessageHeader{
		TotalLength:  uint32(len(authMessageBytes)) + 16,
		HeaderLength: 16,
		Type:         MessageTypeUncompressedNormal,
		OpCode:       OpAuth,
		SequenceID:   sequenceID,
	}

	heartbeatHeader := MessageHeader{
		TotalLength:  uint32(16),
		HeaderLength: 16,
		Type:         MessageTypeUncompressedNormal,
		OpCode:       OpHeartbeat,
		SequenceID:   sequenceID,
	}

	// Write auth message
	if err := authMessageHeader.Write(conn); err != nil {
		return err
	}
	if _, err := conn.Write(authMessageBytes); err != nil {
		return err
	}

	// Send heartbeat every 20 seconds, to give some headroom and avoid the connection being
	// terminated by the server
	ticker := time.NewTicker(time.Second * 20)
	defer ticker.Stop()

	for {
		if err := heartbeatHeader.Write(conn); err != nil {
			return err
		}
		sequenceID++
		heartbeatHeader.SequenceID = sequenceID

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			continue
		}
	}
}

func connect(endpoint *bilibili.LiveEndpoint) (net.Conn, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)))
	if err != nil {
		return nil, err
	}
	return conn, nil
}
