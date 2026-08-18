package server

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/khoi/kuhhandel/internal/game"
)

func newIdentity(name string) (game.Identity, string, error) {
	name = strings.TrimSpace(name)
	if !validName(name) {
		return game.Identity{}, "", protocolError("invalid_player", "name must contain 1 to 50 printable bytes")
	}
	playerID, err := randomID("player_", 12)
	if err != nil {
		return game.Identity{}, "", err
	}
	token, err := randomID("session_", 24)
	if err != nil {
		return game.Identity{}, "", err
	}
	return game.Identity{ID: playerID, AuthHash: game.HashToken(token), Name: name}, token, nil
}

func validName(name string) bool {
	if name == "" || len(name) > 50 || !utf8.ValidString(name) {
		return false
	}
	for _, character := range name {
		if character != ' ' && !unicode.IsGraphic(character) {
			return false
		}
	}
	return true
}

func randomID(prefix string, size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(bytes), nil
}

func randomSeed() (uint64, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(bytes[:]), nil
}
