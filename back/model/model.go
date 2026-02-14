package model

import (
	"go-ping-pong/entities"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var QtdPixels = GetQtdPixels()

var WebsocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var WsClientesGrid = entities.ClientManager{
	Clients: make(map[string]*entities.WsCliente),
}

var Grid = entities.WsGrid{
	GridCores: make(map[int]string, QtdPixels),
}

var OtpManager = entities.OtpManager{
	OtpMap: make(entities.OtpMap),
}

func GetQtdPixels() int {
	value := GoDotEnvVariable("QTD_PIXELS")
	if value == "" {
		return 0 // ou outro valor padrão
	}

	pixels, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Erro ao converter QTD_PIXELS='%s', usando valor padrão 0", value)
		return 0
	}
	return pixels
}

func GoDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func PopularCorGrid(cor string) {
	Grid.Lock()
	defer Grid.Unlock()

	if Grid.UltimoAlterado == QtdPixels {
		Grid.UltimoAlterado = 0
	}

	cor = strings.Trim(cor, "\"")

	Grid.GridCores[Grid.UltimoAlterado] = cor
	Grid.UltimoAlterado++
}

func GetTokenWs(usuario entities.Usuario) (string, time.Time, error) {
	generatedToken, expiresAt, err := CreateOTPWs(usuario)

	return generatedToken, expiresAt, err
}

func CreateOTPWs(usuario entities.Usuario) (string, time.Time, error) {
	o := entities.OTP{
		Key:         uuid.NewString(),
		Created:     time.Now(),
		IdUsuario:   usuario.IdUsuario,
		NomeUsuario: usuario.NomeUsuario,
	}

	OtpManager.Lock()
	OtpManager.OtpMap[o.Key] = o
	OtpManager.Unlock()

	return o.Key, o.Created.Add(60 * time.Second), nil
}

func EnviaMensagemWsClienteGrid() {
	var wg sync.WaitGroup

	WsClientesGrid.Lock()
	clientes := make([]*entities.WsCliente, 0, len(WsClientesGrid.Clients))
	for _, c := range WsClientesGrid.Clients {
		clientes = append(clientes, c)
	}
	WsClientesGrid.Unlock()

	for _, c := range clientes {
		wg.Add(1)

		go func(c *entities.WsCliente) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recover goroutine ws grid para usuario %v: Error - %v", c.IdUsuario, r)
				}
			}()

			if c.WsConn != nil {
				c.Lock()
				defer c.Unlock()
				proximoPixel := Grid.UltimoAlterado
				if Grid.UltimoAlterado == QtdPixels {
					proximoPixel = 0
				}
				wsRetorno := entities.WsRetorno{
					GridCores:    Grid.GridCores,
					ProximoPixel: proximoPixel + 1,
				}
				if err := c.WsConn.WriteJSON(wsRetorno); err != nil {
					log.Printf("Erro ao enviar mensagem para usuário: Error - %s", err.Error())
					RemoveCliente(c.Id)
				}
			}
		}(c)
	}
}

func Reader(cliente *entities.WsCliente) {
	EnviaMensagemWsClienteGrid()
	defer func() {
		RemoveCliente(cliente.Id)
	}()
	for {
		_, p, err := cliente.WsConn.ReadMessage()
		if err != nil {
			// log.Println("Read error:", err)
			break
		}

		// log.Println("Received message:", string(p))
		PopularCorGrid(string(p))
		EnviaMensagemWsClienteGrid()
		// if err := cliente.WsConn.WriteMessage(messageType, p); err != nil {
		// 	log.Println("Write error:", err)
		// 	break
		// }
	}
}

func VerificaClientesWs() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Pânico recuperado em VerificaClientesWs: %v", r)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		WsClientesGrid.Lock()
		for id, cliente := range WsClientesGrid.Clients {
			if cliente.WsConn == nil {
				delete(WsClientesGrid.Clients, id)
				continue
			}

			cliente.Lock()
			if err := cliente.WsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = cliente.WsConn.Close()
				delete(WsClientesGrid.Clients, id)
			}
			cliente.Unlock()
		}
		WsClientesGrid.Unlock()
	}
}

func VerifyOTPWs(otp string) entities.OTP {
	OtpManager.Lock()
	defer OtpManager.Unlock()

	otpClient, ok := OtpManager.OtpMap[otp]
	if !ok || otpClient.Created.Add(60*time.Second).Before(time.Now()) {
		return entities.OTP{}
	}

	delete(OtpManager.OtpMap, otp)
	return otpClient
}

func RemoveCliente(client string) {
	WsClientesGrid.Lock()
	defer WsClientesGrid.Unlock()

	if _, ok := WsClientesGrid.Clients[client]; ok {
		WsClientesGrid.Clients[client].WsConn.Close()
		delete(WsClientesGrid.Clients, client)
	}
}

func Retention(retentionPeriod time.Duration) {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		OtpManager.Lock()
		for _, otp := range OtpManager.OtpMap {
			if otp.Created.Add(retentionPeriod).Before(time.Now()) {
				delete(OtpManager.OtpMap, otp.Key)
			}
		}
		OtpManager.Unlock()
	}
}
