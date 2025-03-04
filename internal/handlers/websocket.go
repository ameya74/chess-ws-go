package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"chess-ws-go/internal/config"
	"chess-ws-go/internal/services"

	"github.com/corentings/chess/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Types to manage player sessions and game state
type Player struct {
	Conn     *websocket.Conn
	Color    chess.Color
	Username string
	UserID   string
}

type GameSession struct {
	White       *Player
	Black       *Player
	Game        *chess.Game
	CurrentTurn chess.Color
}

type WebSocketHandler struct {
	sessions       map[string]*GameSession // gameID -> GameSession
	connections    map[*websocket.Conn]bool
	waitingPlayer  *Player // Player waiting for opponent
	mu             sync.Mutex
	messageService *services.MessageService
	gameService    *services.GameService
	config         *config.Config
}

func NewWebSocketHandler(
	messageService *services.MessageService,
	gameService *services.GameService,
	config *config.Config,
) *WebSocketHandler {
	return &WebSocketHandler{
		sessions:       make(map[string]*GameSession),
		connections:    make(map[*websocket.Conn]bool),
		messageService: messageService,
		gameService:    gameService,
		config:         config,
	}
}

func (h *WebSocketHandler) UpgradeHandler(w http.ResponseWriter, r *http.Request) {
	// Extract authentication information from request context
	userID, _ := r.Context().Value("user_id").(string)
	username, _ := r.Context().Value("username").(string)

	// Validate authentication
	if userID == "" || username == "" {
		log.Println("Authentication required for WebSocket connection")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set WebSocket protocol for token authentication if needed
	upgradeHeaders := http.Header{}

	// Upgrade the connection
	conn, err := upgrader.Upgrade(w, r, upgradeHeaders)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Printf("User %s (ID: %s) connected via WebSocket", username, userID)

	h.mu.Lock()
	h.connections[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.connections, conn)
		h.mu.Unlock()
		log.Printf("User %s (ID: %s) disconnected", username, userID)
	}()

	// Start the reader goroutine with authenticated user info
	go h.authenticatedReader(conn, userID, username)
}

// authenticatedReader is a new method that handles messages with user authentication
func (h *WebSocketHandler) authenticatedReader(conn *websocket.Conn, userID string, username string) {
	defer conn.Close()
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		if messageType != websocket.TextMessage {
			continue
		}

		var message struct {
			Type    string `json:"type"`
			Payload struct {
				Move     string  `json:"move"`
				GameID   string  `json:"gameId"`
				Accept   bool    `json:"accept"`
				TimeLeft float64 `json:"timeLeft"`
				Message  string  `json:"message"`
			} `json:"payload"`
		}

		if err := json.Unmarshal(p, &message); err != nil {
			log.Println("Error unmarshaling message:", err)
			continue
		}

		// Use the authenticated username instead of relying on the message
		switch message.Type {
		case "join":
			h.handleJoinGame(conn, username, userID)
		case "move":
			err := h.handleMove(conn, message.Payload.Move, message.Payload.GameID)
			if err != nil {
				h.sendMessage(conn, struct {
					Type    string `json:"type"`
					Payload string `json:"payload"`
				}{Type: "error", Payload: err.Error()})
			}
		case "resign":
			h.handleResign(conn, message.Payload.GameID)
		case "draw_offer":
			h.handleDrawOffer(conn, message.Payload.GameID)
		case "draw_response":
			h.handleDrawResponse(conn, message.Payload.GameID, message.Payload.Accept)
		case "time_update":
			h.handleTimeUpdate(conn, message.Payload.GameID, message.Payload.TimeLeft)
		case "chat":
			h.handleChat(conn, message.Payload.GameID, message.Payload.Message, username)
		case "reconnect":
			h.handleReconnect(conn, message.Payload.GameID, username, userID)
		case "ping":
			h.handlePing(conn)
		default:
			log.Println("Unknown message type:", message.Type)
		}
	}
}

func generateGameID() string {
	return uuid.New().String()
}

func (h *WebSocketHandler) sendMessage(conn *websocket.Conn, message interface{}) {
	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		log.Println("Error getting next writer:", err)
		return
	}
	defer w.Close()

	err = json.NewEncoder(w).Encode(message)
	if err != nil {
		log.Println("Error encoding message:", err)
	}
}

func (h *WebSocketHandler) handleMove(conn *websocket.Conn, moveStr string, gameID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, exists := h.sessions[gameID]
	if !exists {
		return fmt.Errorf("game session not found")
	}

	// Determine player's color
	var playerColor chess.Color
	if conn == session.White.Conn {
		playerColor = chess.White
	} else if conn == session.Black.Conn {
		playerColor = chess.Black
	} else {
		return fmt.Errorf("player not in this game")
	}

	// Check if it's player's turn
	if playerColor != session.CurrentTurn {
		return fmt.Errorf("not your turn")
	}

	err := session.Game.PushMove(moveStr, nil)
	if err != nil {
		return fmt.Errorf("invalid move: %w", err)
	}
	// Switch turns
	session.CurrentTurn = chess.Color(1 - int(session.CurrentTurn))

	// Broadcast the move to both players
	moveMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			Move     string `json:"move"`
			Position string `json:"position"`
			Turn     string `json:"turn"`
		} `json:"payload"`
	}{
		Type: "move",
		Payload: struct {
			Move     string `json:"move"`
			Position string `json:"position"`
			Turn     string `json:"turn"`
		}{
			Move:     moveStr,
			Position: session.Game.Position().String(),
			Turn:     session.CurrentTurn.String(),
		},
	}

	h.sendMessage(session.White.Conn, moveMsg)
	h.sendMessage(session.Black.Conn, moveMsg)

	// Check for game over
	if session.Game.Outcome() != chess.NoOutcome {
		h.handleGameOver(session)
	}

	return nil
}

func (h *WebSocketHandler) handleGameOver(session *GameSession) {
	// Get game outcome and method
	outcome := session.Game.Outcome()
	method := session.Game.Method()

	// Prepare game over message
	gameOverMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			Outcome string `json:"outcome"`
			Method  string `json:"method"`
			Winner  string `json:"winner"`
		} `json:"payload"`
	}{
		Type: "gameOver",
		Payload: struct {
			Outcome string `json:"outcome"`
			Method  string `json:"method"`
			Winner  string `json:"winner"`
		}{
			Outcome: outcome.String(),
			Method:  method.String(),
			Winner:  determineWinner(outcome),
		},
	}

	// Send game over message to both players
	if session.White != nil && session.White.Conn != nil {
		h.sendMessage(session.White.Conn, gameOverMsg)
	}
	if session.Black != nil && session.Black.Conn != nil {
		h.sendMessage(session.Black.Conn, gameOverMsg)
	}
}

func (h *WebSocketHandler) handleJoinGame(conn *websocket.Conn, username string, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	newPlayer := &Player{
		Conn:     conn,
		Username: username,
		UserID:   userID,
	}

	if h.waitingPlayer == nil {
		// First player joins and waits
		h.waitingPlayer = newPlayer
		h.waitingPlayer.Color = chess.White
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{
			Type:    "waiting",
			Payload: "Waiting for opponent...",
		})
	} else {
		// Second player joins, start the game
		gameID := generateGameID()
		newPlayer.Color = chess.Black

		session := &GameSession{
			White:       h.waitingPlayer,
			Black:       newPlayer,
			Game:        chess.NewGame(),
			CurrentTurn: chess.White,
		}

		h.sessions[gameID] = session
		h.waitingPlayer = nil

		// Notify both players that game has started
		gameStartMsg := struct {
			Type    string `json:"type"`
			Payload struct {
				GameID   string `json:"gameId"`
				Color    string `json:"color"`
				Opponent string `json:"opponent"`
			} `json:"payload"`
		}{Type: "gameStart"}

		// Notify white player
		gameStartMsg.Payload.GameID = gameID
		gameStartMsg.Payload.Color = "white"
		gameStartMsg.Payload.Opponent = newPlayer.Username
		h.sendMessage(session.White.Conn, gameStartMsg)

		// Notify black player
		gameStartMsg.Payload.Color = "black"
		gameStartMsg.Payload.Opponent = session.White.Username
		h.sendMessage(session.Black.Conn, gameStartMsg)
	}
}

// Helper function to determine the winner
func determineWinner(outcome chess.Outcome) string {
	switch outcome {
	case chess.WhiteWon:
		return "white"
	case chess.BlackWon:
		return "black"
	case chess.Draw:
		return "draw"
	default:
		return "unknown"
	}
}

// handleResign handles a player resigning from a game
func (h *WebSocketHandler) handleResign(conn *websocket.Conn, gameID string) {
	h.mu.Lock()
	session, exists := h.sessions[gameID]
	h.mu.Unlock()

	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	// Determine player's color
	var playerColor chess.Color
	if conn == session.White.Conn {
		playerColor = chess.White
	} else if conn == session.Black.Conn {
		playerColor = chess.Black
	} else {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Player not in this game"})
		return
	}

	// Use game service to handle resignation
	err := h.gameService.ResignGame(gameID, playerColor)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// Check game over and notify players
	isOver, outcome, method, _ := h.gameService.IsGameOver(gameID)
	if isOver {
		gameOverMsg := struct {
			Type    string `json:"type"`
			Payload struct {
				Outcome string `json:"outcome"`
				Method  string `json:"method"`
				Winner  string `json:"winner"`
			} `json:"payload"`
		}{
			Type: "gameOver",
			Payload: struct {
				Outcome string `json:"outcome"`
				Method  string `json:"method"`
				Winner  string `json:"winner"`
			}{
				Outcome: outcome.String(),
				Method:  method.String(),
				Winner:  determineWinner(outcome),
			},
		}

		// Send game over message to both players
		h.sendMessage(session.White.Conn, gameOverMsg)
		h.sendMessage(session.Black.Conn, gameOverMsg)
	}
}

// handleDrawOffer handles a player offering a draw
func (h *WebSocketHandler) handleDrawOffer(conn *websocket.Conn, gameID string) {
	h.mu.Lock()
	session, exists := h.sessions[gameID]
	h.mu.Unlock()

	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	// Determine player's color
	var playerColor chess.Color
	var opponent *Player
	if conn == session.White.Conn {
		playerColor = chess.White
		opponent = session.Black
	} else if conn == session.Black.Conn {
		playerColor = chess.Black
		opponent = session.White
	} else {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Player not in this game"})
		return
	}

	// Use game service to handle draw offer
	err := h.gameService.OfferDraw(gameID)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// Notify opponent of draw offer
	drawOfferMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			OfferedBy string `json:"offeredBy"`
		} `json:"payload"`
	}{
		Type: "drawOffer",
		Payload: struct {
			OfferedBy string `json:"offeredBy"`
		}{
			OfferedBy: playerColor.String(),
		},
	}

	h.sendMessage(opponent.Conn, drawOfferMsg)
}

// handleDrawResponse handles a player's response to a draw offer
func (h *WebSocketHandler) handleDrawResponse(conn *websocket.Conn, gameID string, accept bool) {
	h.mu.Lock()
	session, exists := h.sessions[gameID]
	h.mu.Unlock()

	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	var err error
	if accept {
		err = h.gameService.AcceptDraw(gameID)
	} else {
		err = h.gameService.DeclineDraw(gameID)
	}

	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// If draw was accepted, notify both players of game over
	if accept {
		isOver, outcome, method, _ := h.gameService.IsGameOver(gameID)
		if isOver {
			gameOverMsg := struct {
				Type    string `json:"type"`
				Payload struct {
					Outcome string `json:"outcome"`
					Method  string `json:"method"`
					Winner  string `json:"winner"`
				} `json:"payload"`
			}{
				Type: "gameOver",
				Payload: struct {
					Outcome string `json:"outcome"`
					Method  string `json:"method"`
					Winner  string `json:"winner"`
				}{
					Outcome: outcome.String(),
					Method:  method.String(),
					Winner:  "draw",
				},
			}

			h.sendMessage(session.White.Conn, gameOverMsg)
			h.sendMessage(session.Black.Conn, gameOverMsg)
		}
	} else {
		// Notify both players that draw was declined
		drawResponseMsg := struct {
			Type    string `json:"type"`
			Payload struct {
				Accepted bool `json:"accepted"`
			} `json:"payload"`
		}{
			Type: "drawResponse",
			Payload: struct {
				Accepted bool `json:"accepted"`
			}{
				Accepted: false,
			},
		}

		h.sendMessage(session.White.Conn, drawResponseMsg)
		h.sendMessage(session.Black.Conn, drawResponseMsg)
	}
}

// handleTimeUpdate handles updating a player's remaining time
func (h *WebSocketHandler) handleTimeUpdate(conn *websocket.Conn, gameID string, timeLeft float64) {
	h.mu.Lock()
	session, exists := h.sessions[gameID]
	h.mu.Unlock()

	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	// Determine player's color
	var playerColor chess.Color
	if conn == session.White.Conn {
		playerColor = chess.White
	} else if conn == session.Black.Conn {
		playerColor = chess.Black
	} else {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Player not in this game"})
		return
	}

	// Update time in game service
	err := h.gameService.UpdateTime(gameID, playerColor, timeLeft)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// Broadcast time update to both players
	timeUpdateMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			Color    string  `json:"color"`
			TimeLeft float64 `json:"timeLeft"`
		} `json:"payload"`
	}{
		Type: "timeUpdate",
		Payload: struct {
			Color    string  `json:"color"`
			TimeLeft float64 `json:"timeLeft"`
		}{
			Color:    playerColor.String(),
			TimeLeft: timeLeft,
		},
	}

	h.sendMessage(session.White.Conn, timeUpdateMsg)
	h.sendMessage(session.Black.Conn, timeUpdateMsg)
}

// handleChat handles a chat message from a player
func (h *WebSocketHandler) handleChat(conn *websocket.Conn, gameID string, message string, username string) {
	h.mu.Lock()
	session, exists := h.sessions[gameID]
	h.mu.Unlock()

	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	// Add chat message to game service
	err := h.gameService.AddChatMessage(gameID, username, message)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// Broadcast chat message to both players
	chatMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			Sender  string `json:"sender"`
			Message string `json:"message"`
		} `json:"payload"`
	}{
		Type: "chat",
		Payload: struct {
			Sender  string `json:"sender"`
			Message string `json:"message"`
		}{
			Sender:  username,
			Message: message,
		},
	}

	h.sendMessage(session.White.Conn, chatMsg)
	h.sendMessage(session.Black.Conn, chatMsg)
}

// handleReconnect handles a player reconnecting to a game
func (h *WebSocketHandler) handleReconnect(conn *websocket.Conn, gameID string, username string, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, exists := h.sessions[gameID]
	if !exists {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Game not found"})
		return
	}

	// Check if username matches either player
	if session.White.Username == username {
		// Update white player's connection
		session.White.Conn = conn
	} else if session.Black.Username == username {
		// Update black player's connection
		session.Black.Conn = conn
	} else {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: "Player not in this game"})
		return
	}

	// Get current game state
	game, err := h.gameService.GetGame(gameID)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	gameState, err := h.gameService.GetGameState(gameID)
	if err != nil {
		h.sendMessage(conn, struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{Type: "error", Payload: err.Error()})
		return
	}

	// Send current game state to reconnected player
	gameStateMsg := struct {
		Type    string `json:"type"`
		Payload struct {
			Position    string  `json:"position"`
			Turn        string  `json:"turn"`
			WhitePlayer string  `json:"whitePlayer"`
			BlackPlayer string  `json:"blackPlayer"`
			WhiteTime   float64 `json:"whiteTime"`
			BlackTime   float64 `json:"blackTime"`
		} `json:"payload"`
	}{
		Type: "gameState",
		Payload: struct {
			Position    string  `json:"position"`
			Turn        string  `json:"turn"`
			WhitePlayer string  `json:"whitePlayer"`
			BlackPlayer string  `json:"blackPlayer"`
			WhiteTime   float64 `json:"whiteTime"`
			BlackTime   float64 `json:"blackTime"`
		}{
			Position:    game.Position().String(),
			Turn:        game.Position().Turn().String(),
			WhitePlayer: gameState.WhitePlayer,
			BlackPlayer: gameState.BlackPlayer,
			WhiteTime:   gameState.TimeControl.WhiteTimeLeft,
			BlackTime:   gameState.TimeControl.BlackTimeLeft,
		},
	}

	h.sendMessage(conn, gameStateMsg)
}

// handlePing responds to ping messages to keep the connection alive
func (h *WebSocketHandler) handlePing(conn *websocket.Conn) {
	h.sendMessage(conn, struct {
		Type string `json:"type"`
	}{Type: "pong"})
}
