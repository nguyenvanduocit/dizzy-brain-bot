package main

import (
	"context"
	"github.com/alitto/pond"
	"github.com/dgraph-io/ristretto"
	"github.com/sashabaranov/go-openai"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

const allowedChatID = -1001905601063

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	telegramToken := os.Getenv("TELEGRAM_API_TOKEN")
	if telegramToken == "" {
		logger.Panic("telegram api token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		logger.Panic("telegram bot api", zap.Error(err))
	}
	bot.Debug = true

	openAiToken := os.Getenv("OPENAI_API_TOKEN")
	if openAiToken == "" {
		logger.Panic("openai api token is empty")
	}
	openAiClient := openai.NewClient(openAiToken)

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
	})

	if err != nil {
		logger.Panic("create cache", zap.Error(err))
	}
	defer cache.Close()

	pool := pond.New(100, 1000)
	defer pool.StopAndWait()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {

		logger.Info("update", zap.Any("update", update))
		// ignore any non-Message Updates
		if update.Message == nil {
			continue
		}

		if update.FromChat().ID != allowedChatID {
			bot.Send(tgbotapi.NewMessage(update.FromChat().ID, "To use this bot please contact @henry_duocnv"))
			continue
		}

		if update.Message.From.IsBot {
			continue
		}

		// make a copy of the message, because we need to pass it to the goroutine
		message := *update.Message

		// handle commands
		if message.IsCommand() {
			pool.Submit(func() {
				handleCommand(cache, openAiClient, bot, message)
			})
			continue
		}

		// handle text messages
		if bot.IsMessageToMe(message) {
			pool.Submit(func() {
				handleTextMessage(cache, openAiClient, bot, message)
			})
			continue
		}
	}

}

var generateSystemInstruction = func(bot *tgbotapi.BotAPI, message tgbotapi.Message) openai.ChatCompletionMessage {
	senderName := message.From.FirstName + " " + message.From.LastName
	return openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "You're a telegram bot, your name is " + bot.Self.UserName + ". You are very funny, but do not talk much, Always answer in Vietnamese, Xưng hô: 'I' = Tao, 'You = mày'. \n\n Here are some context: \n\n Sender name: " + senderName,
	}
}

func handleTextMessage(cache *ristretto.Cache, openAiClient *openai.Client, bot *tgbotapi.BotAPI, message tgbotapi.Message) {

	messages := []openai.ChatCompletionMessage{
		generateSystemInstruction(bot, message),
	}
	if data, ok := cache.Get(message.Chat.ID); ok {
		history := data.([]openai.ChatCompletionMessage)
		messages = append(messages, history...)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message.Text,
	})

	responseMessage := tgbotapi.NewMessage(message.Chat.ID, "I don't know that command")
	responseMessage.ReplyToMessageID = message.MessageID
	resp, err := openAiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
		},
	)

	if err != nil {
		responseMessage.Text = "Err when create chat completion: " + err.Error()
		// remove the last message
		messages = messages[:len(messages)-1]
	} else {
		responseMessage.Text = resp.Choices[0].Message.Content
		messages = append(messages, resp.Choices[0].Message)
	}

	if len(messages) > 5 {
		// remove the oldest message
		messages = messages[1:]
	}

	cache.Set(message.Chat.ID, messages, 0)

	if _, err := bot.Send(responseMessage); err != nil {
		panic(err)
	}
}

func handleCommand(cache *ristretto.Cache, openAiClient *openai.Client, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	switch message.Command() {
	case "reset":
		cache.Del(message.Chat.ID)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Reset successfully"))
	}
}
