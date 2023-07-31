package main

import (
	"fmt"
	"github.com/alitto/pond"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"os"
	"regexp"
	"strconv"
)

var allowedChatIDs = []int64{
	-1001905601063,
	1831420107,
}

var adminID int64 = 1831420107

func sendMessageToAdmin(bot *tgbotapi.BotAPI, message string) {
	msg := tgbotapi.NewMessage(adminID, message)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	if _, err := bot.Send(msg); err != nil {
		panic(err)
	}
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	// health check
	go func() {
		httpPort := os.Getenv("PORT")
		if httpPort == "" {
			httpPort = "8080"
		}

		logger.Info("http server started", zap.String("port", httpPort))

		if err := startHealthServer(httpPort); err != nil {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	// the AI
	telegramToken := os.Getenv("TELEGRAM_API_TOKEN")
	if telegramToken == "" {
		logger.Panic("telegram api token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		logger.Panic("telegram bot api", zap.Error(err))
	}
	bot.Debug = true

	sendMessageToAdmin(bot, "Bot started")
	defer sendMessageToAdmin(bot, "Bot stopped")

	palmApiToken := os.Getenv("PALM_API_KEY")
	if palmApiToken == "" {
		logger.Panic("PALM_API_KEY is empty")
	}
	openAiClient := NewLLMClient(palmApiToken)

	pool := pond.New(100, 1000, pond.PanicHandler(func(r interface{}) {
		logger.Error("goroutine panic", zap.Any("panic", r))
		sendMessageToAdmin(bot, fmt.Sprintf("goroutine panic: %v", r))

	}))
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

		isAllowed := false
		for _, allowedChatID := range allowedChatIDs {
			if update.Message.Chat.ID == allowedChatID {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
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
				handleCommand(openAiClient, bot, message)
			})
			continue
		}

		// handle text messages
		if bot.IsMessageToMe(message) || message.Chat.IsPrivate() {
			pool.Submit(func() {
				handleTextMessage(openAiClient, bot, message)
			})
			continue
		}
	}

}

func getEmojis(text string) []string {
	// Create the regex.
	regex := regexp.MustCompile(`:([\w\-_]+?):`)

	// Find all matches.
	matches := regex.FindAllString(text, -1)

	// Return the matches.
	return matches
}

var stickerFileIDs = map[string]string{
	":smile:": "CAACAgIAAxkBAAEkYw9kx2Qi25myPxVzg5bJwYuBwjbTXAACEw8AAgOJeUrgcx4dwnsMIi8E",
	":angry:": "CAACAgIAAxkBAAEkYzRkx2f5F0VBHOlnRNsymwACY9tP5AACOxEAAu51cEq0-CRa8FRooC8E",
	":cry:":   "CAACAgIAAxkBAAEkYzlkx2hmxBpZbd-yEGxvWjKtM_-39wAC4BMAAqKFaUgNwTe58GadoC8E",
	":sad:":   "CAACAgIAAxkBAAEkYztkx2iPFy0YjYEmtuXpBvbeAWZ4RAAClBQAAudQcUqDwu0W7SYtAi8E",
}

func handleTextMessage(llmClient *LLMClient, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	conversationID := strconv.FormatInt(message.Chat.ID, 10)
	responseMessage, err := llmClient.GenerateText(conversationID, message.From.ID, message.Text)
	if err != nil {
		responseMessage = err.Error()
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, responseMessage)
	msg.ReplyToMessageID = message.MessageID
	// send sticker

	if _, err := bot.Send(msg); err != nil {
		panic(err)
	}

	stickerIds := getEmojis(responseMessage)
	for _, stickerID := range stickerIds {
		fileID, ok := stickerFileIDs[stickerID]
		if !ok {
			continue
		}
		sticker := tgbotapi.NewSticker(message.Chat.ID, tgbotapi.FileID(fileID))
		if _, err := bot.Send(sticker); err != nil {
			panic(err)
		}
	}

}

func handleCommand(llmClient *LLMClient, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	respMessage := tgbotapi.NewMessage(message.Chat.ID, "Reset successfully")
	switch message.Command() {
	case "reset":
		llmClient.ResetConversation(strconv.FormatInt(message.Chat.ID, 10))
		respMessage.Text = "Reset successfully"
	case "context":
		llmClient.SetContext(strconv.FormatInt(message.Chat.ID, 10), message.CommandArguments())
		respMessage.Text = "Set context successfully"
	case "restart":
		respMessage.Text = "Restarting..."
		// lol
		os.Exit(0)
	}

	respMessage.ReplyToMessageID = message.MessageID
	respMessage.Entities = []tgbotapi.MessageEntity{
		{
			Type:   "bot_command",
			Offset: 0,
			Length: len(respMessage.Text),
		},
	}
	if _, err := bot.Send(respMessage); err != nil {
		panic(err)
	}

}
