package llm

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

type BridgeClient struct {
	WsAddr string
	Conn   *websocket.Conn
}

func NewBridgeClient(roomID string, wsEndpoint string) (*BridgeClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsEndpoint+"?room="+roomID+"&username=ai-commit", nil)
	if err != nil {
		return nil, err
	}

	client := &BridgeClient{
		WsAddr: wsEndpoint,
		Conn:   conn,
	}

	return client, nil
}

func (b *BridgeClient) Close() {
	b.Conn.Close()
}

func (b *BridgeClient) ChatComplete(ctx context.Context, messages []*Message) (string, error) {

	painMessage := ""

	for _, message := range messages {
		painMessage = message.Role + ":" + message.Content
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":      "generateAnswer",
		"recipient": "chatgpt",
		"message":   painMessage,
	})

	b.Conn.WriteMessage(websocket.TextMessage, payload)

	var buf []byte
	var err error
	var responseMessage string

	for {
		if _, buf, err = b.Conn.ReadMessage(); err != nil {
			return "", err
		}

		messageType := gjson.GetBytes(buf, "type").String()
		switch messageType {
		case "generateAnswer/stream":
			responseMessage = gjson.GetBytes(buf, "message").String()
		case "generateAnswer/done":
			break
		}
	}

	return responseMessage, nil
}

func (b *BridgeClient) SingleQuestion(question string) (string, error) {
	return b.ChatComplete(context.Background(), []*Message{
		{
			Role:    "user",
			Content: question,
		},
	})
}
