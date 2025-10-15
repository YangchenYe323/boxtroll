package live

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"

	"github.com/andybalholm/brotli"
	"github.com/rs/zerolog/log"
)

// ReadMessages read messages from the given input stream and return a list of messages.
//
// It only errors out if the input stream is closed, if it encounters unknown or
// unparsable messages, it logs error and skip the message. It can return a nil or empty
// slice.
func ReadMessages(in io.Reader) ([]*Message, error) {
	// Fetch a header
	var header MessageHeader
	if err := header.Read(in); err != nil {
		return nil, err
	}

	// Fetch the entire message
	var buf bytes.Buffer
	buf.Grow(int(header.TotalLength) - int(header.HeaderLength))
	if _, err := io.CopyN(&buf, in, int64(header.TotalLength)-int64(header.HeaderLength)); err != nil {
		return nil, err
	}

	// NOTE: We don't handle heartbeat and auth reply for now
	if header.OpCode == OpHeartbeatReply {
		log.Debug().Msg("收到心跳回复消息")
		return nil, nil
	}
	if header.OpCode == OpAuthReply {
		log.Debug().Msg("收到认证回复消息")
		return nil, nil
	}

	var messages []*Message
	switch header.Type {
	case MessageTypeUncompressedNormal:
		// Single uncompressed message
		message, err := parseMessage(buf.Bytes())
		if err != nil {
			// Ignore the error. The error is logged in parseMessage and should not
			// cause the function to fail, which aborts the stream.
			return nil, nil
		}
		return []*Message{message}, nil
	case MessageTypeUncompressedOperation:
		log.Debug().Msg("跳过认证或心跳包消息")
		return nil, nil
	case MessageTypeZlibNormal:
		// Multiple zlib compressed messages
		var decompressed bytes.Buffer
		reader, err := zlib.NewReader(bytes.NewReader(buf.Bytes()))
		if err != nil {
			// If this error happens, it means we get an invalid zlib compressed message.
			// Log the error and return.
			log.Err(err).Msg("读取 zlib 压缩消息失败, 获取到不合法的zlib压缩消息")
			return nil, nil
		}
		defer reader.Close()
		if _, err := io.Copy(&decompressed, reader); err != nil {
			log.Err(err).Msg("读取 zlib 压缩消息失败, 获取到不合法的zlib压缩消息")
			return nil, nil
		}
		messageReader := bytes.NewReader(decompressed.Bytes())
		for messageReader.Len() > 0 {
			message, err := ReadMessages(messageReader)
			if err != nil {
				return nil, err
			}
			messages = append(messages, message...)
		}
	case MessageTypeBrotliNormal:
		// Multiple brotli compressed messages
		var decompressed bytes.Buffer
		reader := brotli.NewReader(bytes.NewReader(buf.Bytes()))
		if _, err := io.Copy(&decompressed, reader); err != nil {
			log.Err(err).Msg("读取 brotli 压缩消息失败, 获取到不合法的brotli压缩消息")
			return nil, nil
		}
		messageReader := bytes.NewReader(decompressed.Bytes())
		for messageReader.Len() > 0 {
			message, err := ReadMessages(messageReader)
			if err != nil {
				return nil, err
			}
			messages = append(messages, message...)
		}
	default:
		log.Warn().Msgf("未知的弹幕信息类型: %d", header.Type)
	}

	return messages, nil
}

func parseMessage(bytes []byte) (*Message, error) {
	type dummyMessage struct {
		Cmd string `json:"cmd"`
	}
	type giftMessage struct {
		Cmd  string           `json:"cmd"`
		Data *SendGiftMessage `json:"data"`
	}

	// First unmarshal the bytes into a dummy message to extract the cmd, and then
	// re-unmarshal the bytes into the actual message type depending on the cmd
	var dummy dummyMessage
	if err := json.Unmarshal(bytes, &dummy); err != nil {
		log.Err(err).Str("msg", string(bytes)).Msg("弹幕消息解析失败")
		return nil, err
	}

	switch dummy.Cmd {
	case "SEND_GIFT":
		var message giftMessage
		if err := json.Unmarshal(bytes, &message); err != nil {
			log.Err(err).Str("msg", string(bytes)).Msg("SEND_GIFT 消息解析失败")
			return nil, err
		}
		return &Message{
			Cmd:      dummy.Cmd,
			SendGift: message.Data,
		}, nil
	default:
		log.Debug().Str("msg", string(bytes)).Msgf("弹幕消息 %s 还未实现", dummy.Cmd)
		return nil, nil
	}
}
