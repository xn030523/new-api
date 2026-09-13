package tgclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

// Client is a minimal Telegram Bot API client.
type Client struct {
	Token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Telegram Bot API client with the given bot token.
func NewClient(token string) *Client {
	return &Client{
		Token:   token,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithBaseURL points the client at a custom API base URL so tests
// can run against a local Telegram API stub instead of api.telegram.org.
func NewClientWithBaseURL(token, baseURL string) *Client {
	c := NewClient(token)
	c.baseURL = strings.TrimSuffix(baseURL, "/")
	return c
}

// Update represents an incoming update from the getUpdates endpoint.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID       int    `json:"message_id"`
	From            *User  `json:"from,omitempty"`
	Chat            Chat   `json:"chat"`
	Text            string `json:"text,omitempty"`
	MessageThreadID int    `json:"message_thread_id,omitempty"`
	Date            int64  `json:"date"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

// CallbackQuery represents an incoming callback query from an inline keyboard.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from,omitempty"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// BotCommand represents a bot command for setMyCommands.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// InlineKeyboardMarkup represents an inline keyboard attached to a message.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton represents one button in an inline keyboard.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

// apiResponse is the standard Telegram API response envelope.
type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

func (c *Client) endpoint(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.Token, method)
}

// postJSON sends a JSON POST request and checks the API-level response.
func (c *Client) postJSON(method string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("tgclient: marshal %s payload: %w", method, err)
	}
	resp, err := c.httpClient.Post(c.endpoint(method), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("tgclient: %s request failed: %w", method, err)
	}
	defer resp.Body.Close()
	return c.checkResponse(method, resp)
}

// postForm sends a multipart/form-data POST request and checks the API-level response.
func (c *Client) postForm(method string, body *bytes.Buffer, contentType string) error {
	resp, err := c.httpClient.Post(c.endpoint(method), contentType, body)
	if err != nil {
		return fmt.Errorf("tgclient: %s request failed: %w", method, err)
	}
	defer resp.Body.Close()
	return c.checkResponse(method, resp)
}

func (c *Client) checkResponse(method string, resp *http.Response) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("tgclient: read %s response: %w", method, err)
	}
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("tgclient: parse %s response (status %d): %w", method, resp.StatusCode, err)
	}
	if !apiResp.OK {
		return fmt.Errorf("tgclient: %s failed (status %d): %s", method, resp.StatusCode, apiResp.Description)
	}
	return nil
}

// SendMessage sends a plain text message. threadID 0 means no message thread.
func (c *Client) SendMessage(chatID int64, text string, threadID int) error {
	return c.sendMessage(chatID, text, threadID, "", nil)
}

// SendHTML sends a message with HTML parse mode. threadID 0 means no message thread.
func (c *Client) SendHTML(chatID int64, html string, threadID int) error {
	return c.sendMessage(chatID, html, threadID, "HTML", nil)
}

// SendMessageWithKeyboard sends a message with an inline keyboard markup.
func (c *Client) SendMessageWithKeyboard(chatID int64, text string, threadID int, keyboard *InlineKeyboardMarkup) error {
	return c.sendMessage(chatID, text, threadID, "", keyboard)
}

func (c *Client) sendMessage(chatID int64, text string, threadID int, parseMode string, keyboard *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if threadID != 0 {
		payload["message_thread_id"] = threadID
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	return c.postJSON("sendMessage", payload)
}

// SendPhoto sends a photo from raw bytes with an optional caption.
func (c *Client) SendPhoto(chatID int64, photo []byte, caption string, threadID int) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("tgclient: write chat_id field: %w", err)
	}
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("tgclient: write caption field: %w", err)
		}
	}
	if threadID != 0 {
		if err := writer.WriteField("message_thread_id", strconv.Itoa(threadID)); err != nil {
			return fmt.Errorf("tgclient: write message_thread_id field: %w", err)
		}
	}
	part, err := writer.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		return fmt.Errorf("tgclient: create photo form file: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("tgclient: write photo bytes: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("tgclient: close multipart writer: %w", err)
	}
	return c.postForm("sendPhoto", &body, writer.FormDataContentType())
}

// EditMessageText edits the text of a previously sent message.
func (c *Client) EditMessageText(chatID int64, messageID int, text string) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	return c.postJSON("editMessageText", payload)
}

// AnswerCallbackQuery answers a callback query, optionally showing text to the user.
func (c *Client) AnswerCallbackQuery(callbackQueryID string, text string) error {
	payload := map[string]any{
		"callback_query_id": callbackQueryID,
	}
	if text != "" {
		payload["text"] = text
	}
	return c.postJSON("answerCallbackQuery", payload)
}

// GetUpdates fetches pending updates via long polling.
func (c *Client) GetUpdates(offset int64, timeout int) ([]Update, error) {
	params := url.Values{}
	if offset != 0 {
		params.Set("offset", strconv.FormatInt(offset, 10))
	}
	params.Set("timeout", strconv.Itoa(timeout))

	reqURL := c.endpoint("getUpdates")
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	// Long polling may exceed the default client timeout; use a dedicated client.
	httpClient := &http.Client{Timeout: time.Duration(timeout+10) * time.Second}
	resp, err := httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("tgclient: getUpdates request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tgclient: read getUpdates response: %w", err)
	}
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("tgclient: parse getUpdates response (status %d): %w", resp.StatusCode, err)
	}
	if !apiResp.OK {
		return nil, fmt.Errorf("tgclient: getUpdates failed (status %d): %s", resp.StatusCode, apiResp.Description)
	}
	var updates []Update
	if err := json.Unmarshal(apiResp.Result, &updates); err != nil {
		return nil, fmt.Errorf("tgclient: parse updates result: %w", err)
	}
	return updates, nil
}

// SetMyCommands registers the bot's command list.
func (c *Client) SetMyCommands(commands []BotCommand) error {
	payload := map[string]any{
		"commands": commands,
	}
	return c.postJSON("setMyCommands", payload)
}
