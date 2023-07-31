package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
)

type LLMClient struct {
	PalmAPIKey    string
	Conversations map[string]*MessagePrompt
}

const defaultContext = "You are a helpful personal assistant: DizzyBot.\n" +
	"You are wise, you are in a group chat of two member: henry_duocnv and TrucXinh, they are a couple.\n" +
	"You'll do anything to answer their questions, when you do not know, just say you don't know, do not makeup your answer.\n" +
	"Use stickerID instead of emoji, use sticker as much as possible. Make the conversion as natural, usual as possible.\n" +
	"Make the conversation open and fun. Provide more information, and always think twice before you say something.\n" +
	"Response in plain text.\n" +
	"Context: stickerID (:haha:, :smile:, :sad:, :angry:, :cry:)"

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
	Candidates []*Message       `json:"candidates"`
	Messages   []*Message       `json:"messages"`
	Filters    []*ContentFilter `json:"filters"`
	Error      *GenerateError   `json:"error,omitempty"`
}

type ContentFilter struct {
	Reason  string  `json:"reason"`
	Message Message `json:"message"`
}

type MessagePrompt struct {
	Context  string     `json:"context"`
	Messages []*Message `json:"messages"`
	Examples []*Example `json:"examples"`
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
			Context: context,
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

func (c *LLMClient) GenerateText(conversationID string, authorID int64, message string) (string, error) {
	if c.Conversations[conversationID] == nil {
		c.Conversations[conversationID] = &MessagePrompt{
			Context: defaultContext,
		}
	}

	cloned := *c.Conversations[conversationID]
	newUserMessage := &Message{
		Author:  strconv.FormatInt(authorID, 10),
		Content: message,
	}
	cloned.Messages = append(c.Conversations[conversationID].Messages, newUserMessage)

	// Create the payload.
	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(GenerateMessageRequest{
		Prompt:         cloned,
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

	// Check the response code.
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}

	if resp.Filters != nil {
		return "", errors.New(resp.Filters[0].Message.Content)
	}

	// Update the conversation.

	c.Conversations[conversationID].Messages = append(cloned.Messages, resp.Candidates[0])

	if len(c.Conversations[conversationID].Messages) > 5 {
		summaryMessage, err := c.SummaryConversation(strconv.FormatInt(authorID, 10), conversationID)
		if err != nil {
			return "", err
		}
		c.Conversations[conversationID].Messages = []*Message{
			summaryMessage,
		}
	}

	return resp.Candidates[0].Content, nil
}

// summaryConversation returns a summary of the conversation.
func (c *LLMClient) SummaryConversation(authorID, conversationID string) (*Message, error) {
	if c.Conversations[conversationID] == nil {
		return nil, errors.New("conversation not found")
	}

	cloned := *c.Conversations[conversationID]
	newUserMessage := &Message{
		Author:  authorID,
		Content: "Summarize the conversation, make it as detailed as possible.",
	}
	cloned.Messages = append(c.Conversations[conversationID].Messages, newUserMessage)

	// Create the payload.
	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(GenerateMessageRequest{
		Prompt:         cloned,
		Temperature:    0.5,
		CandidateCount: 1,
	})
	if err != nil {
		return nil, err
	}

	// Create the request.
	req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta2/models/chat-bison-001:generateMessage?key="+c.PalmAPIKey, payload)
	if err != nil {
		return nil, err
	}

	// Set the content type header.
	req.Header.Set("Content-Type", "application/json")

	// Make the request.
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read the response body.
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response.
	var resp GenerateMessageResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	// Check the response code.
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}

	if resp.Filters != nil {
		return nil, errors.New(resp.Filters[0].Message.Content)
	}

	// Update the conversation.
	return resp.Candidates[0], nil
}

type GenerateError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
