package services

import (
	"fmt"
	"sync"

	"github.com/corentings/chess/v2"
	"github.com/gorilla/websocket"
)

type MessageService struct {
	gameService     *GameService
	connToGameID    map[*websocket.Conn]string
	mu              sync.Mutex
	messageChannels map[*websocket.Conn]chan interface{}
}

func NewMessageService(gameService *GameService) *MessageService {
	return &MessageService{
		gameService:     gameService,
		connToGameID:    make(map[*websocket.Conn]string),
		messageChannels: make(map[*websocket.Conn]chan interface{}),
	}
}

func (s *MessageService) GetGame(conn *websocket.Conn) *chess.Game {
	s.mu.Lock()
	gameID, ok := s.connToGameID[conn]
	s.mu.Unlock()

	if !ok {
		return nil
	}

	game, err := s.gameService.GetGame(gameID)
	if err != nil {
		return nil
	}
	return game
}

func (s *MessageService) GetGameState(conn *websocket.Conn) interface{} {
	s.mu.Lock()
	gameID, ok := s.connToGameID[conn]
	s.mu.Unlock()

	if !ok {
		return nil
	}

	game, err := s.gameService.GetGame(gameID)
	if err != nil {
		return nil
	}

	return struct {
		Board string `json:"board"`
	}{
		Board: game.Position().Board().Draw(),
	}
}

func (s *MessageService) GetGameResult(conn *websocket.Conn) string {
	s.mu.Lock()
	gameID, ok := s.connToGameID[conn]
	s.mu.Unlock()

	if !ok {
		return ""
	}

	game, err := s.gameService.GetGame(gameID)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s by %s", game.Outcome(), game.Method())
}

func (s *MessageService) GetMessageChannel(conn *websocket.Conn) chan interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.messageChannels[conn]
	if !ok {
		ch = make(chan interface{})
		s.messageChannels[conn] = ch
	}
	return ch
}

func (s *MessageService) IsGameOver(conn *websocket.Conn) bool {
	s.mu.Lock()
	gameID, ok := s.connToGameID[conn]
	s.mu.Unlock()

	if !ok {
		return false
	}

	game, err := s.gameService.GetGame(gameID)
	if err != nil {
		return false
	}
	return game.Outcome() != chess.NoOutcome
}

// AssociateConnection associates a WebSocket connection with a game ID
func (s *MessageService) AssociateConnection(conn *websocket.Conn, gameID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connToGameID[conn] = gameID
}

// RemoveConnection removes a WebSocket connection from the service
func (s *MessageService) RemoveConnection(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connToGameID, conn)
	delete(s.messageChannels, conn)
}

// GetActiveConnectionsCount returns the number of active WebSocket connections
func (s *MessageService) GetActiveConnectionsCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.connToGameID)
}
