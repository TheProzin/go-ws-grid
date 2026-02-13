package main

import (
	"crypto/tls"
	"encoding/json"
	"go-ping-pong/entities"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

func GoDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetPrefix("go-ping-pong: ")
	log.SetFlags(log.Ldate | log.Ltime)
	var err error

	go Retention(60 * time.Second)
	go VerificaClientesWs()

	r := mux.NewRouter()

	r.HandleFunc("/wsNotificacao", WsEndpointHandler)
	r.HandleFunc("/getTokenWsNotificacao", GetTokenWsHandler)

	port := ":" + GoDotEnvVariable("SERVER_PORT")
	srv := &http.Server{
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		Handler:      r, // Use the router
		TLSConfig:    &tls.Config{},
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Println("Erro no server: ", err)
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println("Recovered from panic na main:", err)
		}
	}()
}

var WebsocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var WsClientesGrid = entities.ClientManager{
	Clients: make(map[string]*entities.WsCliente),
}

var Grid = entities.WsGrid{
	GridCores: make(map[int]string, 25),
}

var OtpManager = entities.OtpManager{
	OtpMap: make(entities.OtpMap),
}

func Retention(retentionPeriod time.Duration) {
	ticker := time.NewTicker(5000 * time.Millisecond)
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

func VerificaClientesWs() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Pânico recuperado em VerificaClientesWs: %v", r)
		}
	}()

	ticker := time.NewTicker(3000 * time.Second)
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
	if !ok || otpClient.Created.Add(600*time.Second).Before(time.Now()) {
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

func PopularCorGrid(cor string) {
	Grid.Lock()
	defer Grid.Unlock()

	if Grid.UltimoAlterado == 25 {
		Grid.UltimoAlterado = 0
	}

	Grid.GridCores[Grid.UltimoAlterado] = cor
	Grid.UltimoAlterado++
}

func WsEndpoint(w http.ResponseWriter, r *http.Request, idUsuario string) {
	conn, err := WebsocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	uniqueId := uuid.NewString()
	newClient := &entities.WsCliente{

		IdUsuario: idUsuario,
		WsConn:    conn,
		Id:        uniqueId,
	}

	WsClientesGrid.Lock()
	WsClientesGrid.Clients[uniqueId] = newClient
	WsClientesGrid.Unlock()

	if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		log.Println("Error sending initial message:", err)
		return
	}

	Reader(newClient)
}

func WsEndpointHandler(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	otp := req.URL.Query().Get("otp")
	if otp == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	otpClient := VerifyOTPWs(otp)

	WsEndpointRegistrar(w, req, otpClient)
}

func WsEndpointRegistrar(w http.ResponseWriter, r *http.Request, otpClient entities.OTP) {
	conn, err := WebsocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	uniqueId := uuid.NewString()
	newClient := &entities.WsCliente{
		Id:        uniqueId,
		WsConn:    conn,
		IdUsuario: otpClient.IdUsuario,
	}

	WsClientesGrid.Lock()
	WsClientesGrid.Clients[uniqueId] = newClient
	WsClientesGrid.Unlock()

	if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		log.Println("Error sending initial message:", err)
		return
	}

	Reader(newClient)
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

	return o.Key, o.Created.Add(600 * time.Second), nil
}

func GetTokenWsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if req.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var requestData entities.Usuario

	err := json.NewDecoder(req.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Erro ao decodificar requisição: "+err.Error(), http.StatusBadRequest)
		return
	}

	if requestData.IdUsuario == "" {
		http.Error(w, "ID do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	if requestData.NomeUsuario == "" {
		http.Error(w, "Nome do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	generatedToken, expiresAt, err := GetTokenWs(requestData)
	if err != nil {
		http.Error(w, "Erro ao gerar token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := entities.LoginResponse{
		Token:         generatedToken,
		DataExpiracao: expiresAt,
	}

	json.NewEncoder(w).Encode(response)
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
					log.Printf("Recover goroutine ws notificacao para usuario %v: Error - %v", c.IdUsuario, r)
				}
			}()

			if c.WsConn != nil {
				c.Lock()
				defer c.Unlock()
				if err := c.WsConn.WriteJSON(Grid.GridCores); err != nil {
					log.Printf("Erro ao enviar mensagem para usuário: Error - %s", err.Error())
					RemoveCliente(c.Id)
				}
			}
		}(c)
	}
}
