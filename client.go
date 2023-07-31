package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type LLMClient struct {
	PalmAPIKey    string
	Conversations map[string]*MessagePrompt
}

const defaultContext = "You are a helpful personal assistant: DizzyBot, you are in a group chat, there are two member: Duoc(a develper) and Truc Xinh (a designer), they are a couple. Your will help to answer their questions in a funny and helpful, use emoji as much as you can"

func NewLLMClient(palmAPIKey string) *LLMClient {
	return &LLMClient{
		Conversations: map[string]*MessagePrompt{},
		PalmAPIKey:    palmAPIKey,
	}
}

type GenerateMessageRequest struct {
	Prompt         MessagePrompt `json:"prompt"`
	Temperature    float64       `json:"temperature"`
	CandidateCount int           `json:"candidate_count"`
}

type GenerateMessageResponse struct {
	Candidates []Message       `json:"candidates"`
	Messages   []Message       `json:"messages"`
	Filters    []ContentFilter `json:"filters"`
}

type ContentFilter struct {
	Reason  string  `json:"reason"`
	Message Message `json:"message"`
}

type MessagePrompt struct {
	Context  string    `json:"context"`
	Messages []Message `json:"messages"`
	Examples []Example `json:"examples"`
}

type Message struct {
	Author  string `json:"author"`
	Content string `json:"content"`
}

type Example struct {
	Input  Message `json:"input"`
	Output Message `json:"output"`
}

// SetContext sets the context for the conversation.
func (c *LLMClient) SetContext(conversationID, context string) {
	if c.Conversations[conversationID] == nil {
		c.Conversations[conversationID] = &MessagePrompt{
			Context: defaultContext,
		}
	} else {
		c.Conversations[conversationID].Context = context
	}

}

// ResetConversation resets the conversation.
func (c *LLMClient) ResetConversation(conversationID string) {
	c.Conversations[conversationID] = &MessagePrompt{
		Context: defaultContext,
	}
}

func (c *LLMClient) GenerateText(conversationID, author, message string) (string, error) {
	if c.Conversations[conversationID] == nil {
		c.Conversations[conversationID] = &MessagePrompt{
			Context: defaultContext,
		}
	}

	c.Conversations[conversationID].Messages = append(c.Conversations[conversationID].Messages, Message{
		Author:  author,
		Content: message,
	})

	// Create the payload.
	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(GenerateMessageRequest{
		Prompt:         *c.Conversations[conversationID],
		Temperature:    0.5,
		CandidateCount: 1,
	})
	if err != nil {
		return "", err
	}

	// Create the request.
	req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta2/models/chat-bison-001:generateMessage?key="+c.PalmAPIKey, payload)
	if err != nil {
		return "", err
	}

	// Set the content type header.
	req.Header.Set("Content-Type", "application/json")

	// Make the request.
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	// Check the response code.
	if response.StatusCode != http.StatusOK {
		return "", err
	}

	// Read the response body.
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// Unmarshal the response.
	var resp GenerateMessageResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return "", err
	}

	// Update the conversation.
	c.Conversations[conversationID].Messages = resp.Messages

	return resp.Candidates[0].Content, nil
}
