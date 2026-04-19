package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"time"
)

var PORT = ":" + getEnv("SERVER_PORT", "7777")
var MAX_PLAYERS = getEnvInt("MAX_PLAYERS", 10)

const TIMEOUT = 30 * time.Second

type Player struct {
	ID       uint8
	Addr     *net.UDPAddr
	LastSeen time.Time
	Nick     string
	IsHost   bool
	X, Y, Z  float32
	RotY     float32
}

var players = make(map[string]*Player)
var nextID uint8 = 1
var hostKey string = ""

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		var n int
		fmt.Sscanf(val, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return fallback
}

func float32ToBytes(f float32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(f))
	return b
}

func main() {
	addr, err := net.ResolveUDPAddr("udp", PORT)
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Printf("[MSCServer] Serwer uruchomiony na porcie %s\n", PORT)
	fmt.Printf("[MSCServer] Max graczy: %d\n", MAX_PLAYERS)

	go cleanupLoop(conn)

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		go handlePacket(conn, remoteAddr, data)
	}
}

func handlePacket(conn *net.UDPConn, addr *net.UDPAddr, data []byte) {
	if len(data) == 0 {
		return
	}

	key := addr.String()
	packetType := data[0]

	switch packetType {

	case 0x01: // Connect z nickiem
		if _, exists := players[key]; exists {
			// Juz polaczony — odnow
			players[key].LastSeen = time.Now()
			conn.WriteToUDP([]byte{0x01, players[key].ID}, addr)
			return
		}
		if len(players) >= MAX_PLAYERS {
			conn.WriteToUDP([]byte{0xFF}, addr)
			return
		}

		nick := "Gracz"
		if len(data) >= 3 {
			nickLen := int(data[1])
			if len(data) >= 2+nickLen {
				nick = string(data[2 : 2+nickLen])
			}
		}

		isHost := len(players) == 0
		player := &Player{
			ID:       nextID,
			Addr:     addr,
			LastSeen: time.Now(),
			Nick:     nick,
			IsHost:   isHost,
		}
		players[key] = player
		if isHost {
			hostKey = key
		}
		nextID++

		fmt.Printf("[MSCServer] Gracz %d dolaczyl: %s (Nick: %s, Host: %v)\n", player.ID, addr, nick, isHost)

		// Wyslij welcome z ID i czy jest hostem
		hostByte := byte(0)
		if isHost {
			hostByte = 1
		}
		conn.WriteToUDP([]byte{0x01, player.ID, hostByte}, addr)

		// Powiadom innych o nowym graczu
		nickBytes := []byte(nick)
		joinPacket := append([]byte{0x03, player.ID, byte(len(nickBytes))}, nickBytes...)
		broadcast(conn, addr, joinPacket)

		// Wyslij nowemu graczowi liste obecnych graczy
		for _, p := range players {
			if p.ID != player.ID {
				pNickBytes := []byte(p.Nick)
				listPacket := append([]byte{0x03, p.ID, byte(len(pNickBytes))}, pNickBytes...)
				conn.WriteToUDP(listPacket, addr)
			}
		}

	case 0x02: // Pozycja Satsumy
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			if len(data) >= 18 {
				broadcast(conn, addr, data)
			}
		}

	case 0x04: // Pozycja ciala gracza
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			broadcast(conn, addr, data)
		}

	case 0x05: // Synchronizacja obiektu
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			broadcast(conn, addr, data)
		}

	case 0x06: // Animacja gracza
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			broadcast(conn, addr, data)
		}

	case 0x07: // Chat
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			// Dodaj nick do pakietu
			nickBytes := []byte(player.Nick)
			chatPacket := append([]byte{0x07, byte(len(nickBytes))}, nickBytes...)
			chatPacket = append(chatPacket, data[1:]...)
			broadcastAll(conn, chatPacket)
		}

	case 0x09: // Ping
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			conn.WriteToUDP([]byte{0x09}, addr)
		}

	case 0x00: // Disconnect
		if player, exists := players[key]; exists {
			fmt.Printf("[MSCServer] Gracz %d rozlaczyl sie: %s\n", player.ID, addr)
			broadcast(conn, addr, []byte{0x00, player.ID})
			delete(players, key)
			if key == hostKey && len(players) > 0 {
				// Przekaz host innemu graczowi
				for k, p := range players {
					p.IsHost = true
					hostKey = k
					conn.WriteToUDP([]byte{0x08, p.ID}, p.Addr) // Nowy host
					fmt.Printf("[MSCServer] Nowy host: %s\n", p.Nick)
					break
				}
			}
		}
	}
}

func broadcast(conn *net.UDPConn, sender *net.UDPAddr, data []byte) {
	for _, player := range players {
		if player.Addr.String() != sender.String() {
			conn.WriteToUDP(data, player.Addr)
		}
	}
}

func broadcastAll(conn *net.UDPConn, data []byte) {
	for _, player := range players {
		conn.WriteToUDP(data, player.Addr)
	}
}

func cleanupLoop(conn *net.UDPConn) {
	for {
		time.Sleep(10 * time.Second)
		now := time.Now()
		for key, player := range players {
			if now.Sub(player.LastSeen) > TIMEOUT {
				fmt.Printf("[MSCServer] Gracz %d timeout: %s\n", player.ID, player.Addr)
				broadcast(conn, player.Addr, []byte{0x00, player.ID})
				delete(players, key)
			}
		}
	}
}
