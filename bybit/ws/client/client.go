package client

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"os"
	"sync"
	"time"
)

const (
	DefaultScheme       = "wss"
	PingInterval        = 20 * time.Second
	PingOperation       = "ping"
	AuthOperation       = "auth"
	ReconnectionRetries = 3
	ReconnectionDelay   = 10 * time.Second
	Public              = "public"
	Private             = "private"
)

var DefaultReqID = randomString(8)

// PingMsg represents the WebSocket ping message format.
type PingMsg struct {
	Op    string `json:"op"`
	ReqId string `json:"req_id,omitempty"`
}

// ChannelType defines the types of channels (public/private) that the WebSocket client can connect to.
type ChannelType string

// Client is the main WebSocket client struct, managing the connection and its state.
type Client struct {
	Conn              *websocket.Conn
	closeOnce         sync.Once
	isClosed          bool
	logger            *log.Logger
	IsTestNet         bool
	ApiKey            string
	ApiSecret         string
	Channel           ChannelType
	Path              string
	Connected         chan struct{}
	OnConnected       func()
	OnConnectionError func(err error)
	Category          string
	MaxActiveTime     string
	wsURL             string // WebSocket URL for dependency injection in tests
	connChan          chan *websocket.Conn
	errorChan         chan error
	once              sync.Once
}

// Connect establishes a WebSocket connection to the server based on the configuration.
func (c *Client) Connect() error {
	c.once.Do(func() {
		if c.isClosed {
			err := errors.New("connection already closed")
			c.handleConnectionError(err)
			return
		}

		url := c.buildURL()
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			c.handleConnectionError(fmt.Errorf("failed to dial %s: %v", url, err))
			return
		}

		c.connChan <- conn
		c.logger.Printf("Connected to %s", url)
		if c.OnConnected != nil {
			c.OnConnected()
		}
		closeOnce(c.Connected) // Close the channel only once

		go c.keepAlive(conn)

		// Authenticate if required
		if c.Channel == Private {
			if err := c.authenticateIfRequired(conn); err != nil {
				return
			}
		}
	})

	select {
	case err := <-c.errorChan:
		return err
	default:
		return nil
	}
}

// buildURL constructs the WebSocket URL based on client configuration.
func (c *Client) buildURL() string {
	if c.wsURL != "" {
		return c.wsURL
	}

	var baseURL string
	if c.IsTestNet {
		baseURL = "stream-testnet.bybit.com"
	} else {
		baseURL = "stream.bybit.com"
	}

	switch c.Channel {
	case Public:
		switch c.Category {
		case "spot":
			return fmt.Sprintf("%s://%s/v5/public/spot", DefaultScheme, baseURL)
		case "usdt_contract", "usdc_contract", "usdc_futures":
			return fmt.Sprintf("%s://%s/v5/public/linear", DefaultScheme, baseURL)
		case "inverse_contract":
			return fmt.Sprintf("%s://%s/v5/public/inverse", DefaultScheme, baseURL)
		case "usdc_option":
			return fmt.Sprintf("%s://%s/v5/public/option", DefaultScheme, baseURL)
		default:
			return fmt.Sprintf("%s://%s/v5/public/linear", DefaultScheme, baseURL) // default to linear (USDT/USDC)
		}
	case Private:
		return fmt.Sprintf("%s://%s/v5/private", DefaultScheme, baseURL)
	default:
		return fmt.Sprintf("%s://%s/v5/public/linear", DefaultScheme, baseURL) // default URL
	}
}

// NewPublicClient initializes a new public WSClient instance.
func NewPublicClient(isTestNet bool, category string) (*Client, error) {
	client := &Client{
		logger:    log.New(os.Stdout, "[WebSocketClient] ", log.LstdFlags),
		IsTestNet: isTestNet,
		Channel:   Public,
		Connected: make(chan struct{}),
		Category:  category,
		connChan:  make(chan *websocket.Conn, 1),
		errorChan: make(chan error, 1),
	}
	DefaultReqID = randomString(8)
	return client, nil
}

// NewPrivateClient initializes a new private WSClient instance.
func NewPrivateClient(apiKey, apiSecret string, isTestNet bool, maxActiveTime string, category string) (*Client, error) {
	client := &Client{
		logger:        log.New(os.Stdout, "[WebSocketClient] ", log.LstdFlags),
		IsTestNet:     isTestNet,
		ApiKey:        apiKey,
		ApiSecret:     apiSecret,
		Channel:       Private,
		Connected:     make(chan struct{}),
		MaxActiveTime: maxActiveTime,
		Category:      category,
		connChan:      make(chan *websocket.Conn, 1),
		errorChan:     make(chan error, 1),
	}
	DefaultReqID = randomString(8)
	return client, nil
}

// authenticateIfRequired authenticates the WebSocket client if the channel is private.
func (c *Client) authenticateIfRequired(conn *websocket.Conn) error {
	if c.Channel == Private {
		expires := fmt.Sprintf("%d", time.Now().UnixMilli()+1000)
		signatureData := fmt.Sprintf("GET/realtime%s", expires)
		signed := GenerateWsSignature(c.ApiSecret, signatureData)
		c.logger.Printf("Authenticating with apiKey %s, expires %s, signed %s", c.ApiKey, expires, signed)
		return c.Authenticate(conn, c.ApiKey, expires, signed)
	}
	return nil
}

// GenerateWsSignature generates a signature for the WebSocket API.
func GenerateWsSignature(apiSecret, data string) string {
	if data == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// keepAlive sends a ping message to the WebSocket server every PingInterval and handles reconnection if the ping fails.
func (c *Client) keepAlive(conn *websocket.Conn) {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.sendPingAndHandleReconnection(conn)
	}
}

// sendPingAndHandleReconnection sends a ping message to the WebSocket server and handles reconnection if the ping fails.
func (c *Client) sendPingAndHandleReconnection(conn *websocket.Conn) {
	if c.isClosed {
		return
	}

	pingMsg := PingMsg{
		ReqId: DefaultReqID,
		Op:    PingOperation,
	}
	jsonData, err := json.Marshal(pingMsg)
	if err != nil {
		c.logger.Printf("Error marshaling ping message: %v", err)
		return
	}

	if err = conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		c.logger.Printf("Error sending ping: %v", err)
		c.handleReconnection()
		return
	}
	c.logger.Println("Ping sent")
}

// Authenticate sends an authentication request to the WebSocket server.
func (c *Client) Authenticate(conn *websocket.Conn, apiKey, expires, signature string) error {
	if c.Channel != Private {
		return errors.New("cannot authenticate on a public channel")
	}
	c.logger.Printf("Authenticating with apiKey %s, expires %s, signed %s", apiKey, expires, signature)
	authRequest := map[string]interface{}{
		"op":   AuthOperation,
		"args": []interface{}{apiKey, expires, signature},
	}
	jsonData, err := json.Marshal(authRequest)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		c.handleConnectionError(err)
		return err
	}
	return nil
}

// Close gracefully closes the WebSocket connection.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.isClosed = true
		c.logger.Println("Connection closed")
		select {
		case conn := <-c.connChan:
			if conn != nil {
				if err := conn.Close(); err != nil && c.OnConnectionError != nil {
					c.OnConnectionError(err)
				}
			}
		default:
		}
	})
}

// randomString generates a random string of specified length.
func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Send sends a message to the WebSocket server.
func (c *Client) Send(message []byte) error {
	if c.isClosed {
		return errors.New("attempt to send message on closed connection")
	}

	select {
	case conn := <-c.connChan:
		if conn == nil {
			log.Println("Connection is nil, attempting to reconnect...")
			if err := c.Connect(); err != nil {
				log.Printf("Reconnection failed: %v", err)
				return err
			}
			conn = <-c.connChan
		}

		if conn == nil {
			return errors.New("connection is still nil after attempting to reconnect")
		}
		fmt.Println(string(message))

		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Error sending message: %v", err)
			return err
		}

		c.connChan <- conn
	default:
		return errors.New("no available connection")
	}

	return nil
}

// Receive listens for a message from the WebSocket server and returns it.
func (c *Client) Receive() ([]byte, error) {
	select {
	case conn := <-c.connChan:
		if conn == nil {
			return nil, errors.New("attempt to receive message on nil connection")
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			return nil, err
		}

		fmt.Println(string(message))
		c.connChan <- conn
		return message, nil
	default:
		return nil, errors.New("no available connection")
	}
}

// handleReconnection attempts to reconnect to the WebSocket server.
func (c *Client) handleReconnection() {
	for i := 0; i < ReconnectionRetries; i++ {
		time.Sleep(ReconnectionDelay)
		if err := c.Connect(); err == nil {
			c.logger.Printf("Reconnection attempt %d successful", i+1)
			return
		}
		c.logger.Printf("Reconnection attempt %d failed", i+1)
	}
}

func (c *Client) handleConnectionError(err error) {
	if c.OnConnectionError != nil {
		c.OnConnectionError(err)
	}
	c.logger.Printf("Connection error: %v", err)
}

// closeOnce ensures the channel is only closed once
func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
		// Channel is already closed
	default:
		close(ch)
	}
}
