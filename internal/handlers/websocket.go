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
}

type GameSession struct {
	White       *Player
	Black       *Player
	Game        *chess.Game
	CurrentTurn chess.Color
}

type WebSocketHandler struct {
	sessions       map[string]*GameSession // gameID -> GameSession
	waitingPlayer  *Player                 // Player waiting for opponent
	mu             sync.Mutex
	messageService *services.MessageService
	config         *config.Config
}

// Modified NewWebSocketHandler
func NewWebSocketHandler(
    messageService *services.MessageService,
    gameService *services.GameService,
    config *config.Config,
) *WebSocketHandler {
    return &WebSocketHandler{
        sessions:       make(map[string]*GameSession),
        messageService: messageService,
        gameService:    gameService,
        config:        config,
    }
}


func (h *WebSocketHandler) UpgradeHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mu.Lock()
	h.connections[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.connections, conn)
		h.mu.Unlock()
	}()

	go h.reader(conn) // Start the reader goroutine
	go h.writer(conn) // Start the writer goroutine
}

func (h *WebSocketHandler) handleJoinGame(conn *websocket.Conn, username string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    newPlayer := &Player{
        Conn:     conn,
        Username: username,
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
        gameID := generateGameID() //TODO :Implement this helper function
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

func (h *WebSocketHandler) reader(conn *websocket.Conn) {
	defer conn.Close()
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break // Exit the loop on error (connection closed)
		}

		if messageType != websocket.TextMessage {
			continue // Ignore non-text messages for now
		}

		var message struct {
			Type    string      `json:"type"`
			Payload interface{} `json:"payload"`
		}

		if err := json.Unmarshal(p, &message); err != nil {
			log.Println("Error unmarshaling message:", err)
			continue
		}

		switch message.Type {
		case "move":
			moveStr, ok := message.Payload.(string)
			if !ok {
				log.Println("Invalid move payload")
				continue
			}
			err := h.handleMove(conn, moveStr, gameID)
			if err != nil {
				h.sendMessage(conn, struct {
					Type    string `json:"type"`
					Payload string `json:"payload"`
				}{Type: "error", Payload: err.Error()})
			}
		// ... other cases
		default:
			log.Println("Unknown message type:", message.Type)
		}
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

    // Make the move
    notation := chess.LongAlgebraicNotation{}
    move, err := notation.Decode(session.Game.Position(), moveStr)
    if err != nil {
        return fmt.Errorf("invalid move format: %w", err)
    }

    err = session.Game.Move(move)
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

func (h *WebSocketHandler) broadcastMessage(message interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.connections {
		h.sendMessage(conn, message)
	}
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
