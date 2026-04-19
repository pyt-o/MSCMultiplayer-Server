package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

var PORT = ":" + getEnv("SERVER_PORT", "7777")

const MAX_PLAYERS_DEFAULT = 10
const TIMEOUT = 30 * time.Second

var MAX_PLAYERS = getEnvInt("MAX_PLAYERS", MAX_PLAYERS_DEFAULT)

type Player struct {
	ID       uint8
	Addr     *net.UDPAddr
	LastSeen time.Time
	X, Y, Z  float32
}

var players = make(map[string]*Player)
var nextID uint8 = 1

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

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		go handlePacket(conn, remoteAddr, buf[:n])
	}
}

func handlePacket(conn *net.UDPConn, addr *net.UDPAddr, data []byte) {
	if len(data) == 0 {
		return
	}
	key := addr.String()
	packetType := data[0]
	switch packetType {
	case 0x01:
		if _, exists := players[key]; !exists {
			if len(players) >= MAX_PLAYERS {
				conn.WriteToUDP([]byte{0xFF}, addr)
				return
			}
			player := &Player{
				ID:       nextID,
				Addr:     addr,
				LastSeen: time.Now(),
			}
			players[key] = player
			nextID++
			fmt.Printf("[MSCServer] Gracz %d dolaczyl: %s\n", player.ID, addr)
			conn.WriteToUDP([]byte{0x01, player.ID}, addr)
		}
	case 0x02:
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			broadcast(conn, addr, data)
		}
	case 0x00:
		if player, exists := players[key]; exists {
			fmt.Printf("[MSCServer] Gracz %d rozlaczyl sie: %s\n", player.ID, addr)
			delete(players, key)
			broadcast(conn, addr, []byte{0x00, player.ID})
		}
	case 0x09:
		if player, exists := players[key]; exists {
			player.LastSeen = time.Now()
			conn.WriteToUDP([]byte{0x09}, addr)
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
